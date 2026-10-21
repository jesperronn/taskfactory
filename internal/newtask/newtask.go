// Package newtask scaffolds one inbox task file. It picks the next free task ID
// across every state directory, derives a slug from the title and writes the
// skeleton to the inbox. It writes only that one file and never commits.
package newtask

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"taskfactory/internal/paths"
)

// maxSlug bounds the slug length so file names stay readable.
const maxSlug = 60

var fileIDPattern = regexp.MustCompile(`^TF-([0-9]{3})-`)

// UsageError marks an unusable title, which the CLI reports as exit code 2.
type UsageError struct{ msg string }

func (e UsageError) Error() string { return e.msg }

// UsageErrorf builds a UsageError; the CLI also uses it for argument errors.
func UsageErrorf(format string, args ...any) UsageError {
	return UsageError{fmt.Sprintf(format, args...)}
}

// Options are the common flags. With DryRun, Would receives the planned action
// and nothing is written. Verbose, when non-nil, receives detail lines.
type Options struct {
	DryRun  bool
	Verbose func(format string, args ...any)
	Would   func(format string, args ...any)
}

// Result names the scaffolded task.
type Result struct {
	ID string
	// Rel is the slash-separated path relative to the state root.
	Rel string
}

// Slug derives a file-name slug from a title: lowercase ASCII letters and
// digits, with every other run of characters turned into one hyphen and no
// leading or trailing hyphen. It is empty when the title has no usable
// characters.
func Slug(title string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
			continue
		}
		pendingHyphen = true
	}
	slug := b.String()
	if len(slug) > maxSlug {
		slug = strings.TrimRight(slug[:maxSlug], "-")
	}
	return slug
}

// NextID returns the lowest TF-NNN above every ID found in a task file name in
// any state directory, the archive included.
func NextID(root string) (string, error) {
	layout := paths.For(root)
	highest := 0
	for _, state := range paths.States {
		entries, err := os.ReadDir(layout.TaskDir(state))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("read %s: %w", layout.TaskDirRel(state), err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if m := fileIDPattern.FindStringSubmatch(entry.Name()); m != nil {
				n, _ := strconv.Atoi(m[1])
				if n > highest {
					highest = n
				}
			}
		}
	}
	if highest >= 999 {
		return "", errors.New("no free task ID: TF-999 is already used")
	}
	return fmt.Sprintf("TF-%03d", highest+1), nil
}

// Skeleton returns the inbox task text: every ready heading in order, one
// criterion with its Check line, and TODO markers where prose is needed.
func Skeleton(id, title string) string {
	return "# " + id + ": " + title + "\n\n" +
		"## Goal\n\nTODO: state the outcome in one or two sentences.\n\n" +
		"## Dependencies\n\nNone\n\n" +
		"## Scope\n\nTODO: list what to change.\n\n" +
		"## Constraints\n\nTODO: list what must not change.\n\n" +
		"## Success criteria\n\n### C1: TODO describe the observable result\n\nCheck: TODO command that exits 0 when the result holds\n\n" +
		"## Verification\n\nTODO: say which commands to run and what to report.\n"
}

// Create writes tasks/inbox/<ID>-<slug>.md for the title. It refuses an empty
// title, a title with a line break and a title with no usable slug characters.
func Create(root, title string, opts Options) (Result, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Result{}, UsageError{"the title must not be empty"}
	}
	if !utf8.ValidString(title) || strings.ContainsAny(title, "\r\n\t") {
		return Result{}, UsageError{"the title must be one line of text without tabs"}
	}
	slug := Slug(title)
	if slug == "" {
		return Result{}, UsageError{fmt.Sprintf("the title %q has no letters or digits to build a file name from", title)}
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Result{}, fmt.Errorf("resolve project root %s: %w", root, err)
	}
	layout := paths.For(resolved)
	id, err := NextID(resolved)
	if err != nil {
		return Result{}, err
	}
	name := id + "-" + slug + ".md"
	res := Result{ID: id, Rel: layout.TaskFileRel("inbox", name)}
	abs := layout.TaskFile("inbox", name)
	if opts.Verbose != nil {
		opts.Verbose("next free task ID %s", id)
		opts.Verbose("write %s", res.Rel)
	}
	if opts.DryRun {
		if opts.Would != nil {
			opts.Would("write %s", res.Rel)
		}
		return res, nil
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return Result{}, fmt.Errorf("create %s: %w", layout.TaskDirRel("inbox"), err)
	}
	f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return Result{}, fmt.Errorf("write %s: %w", res.Rel, err)
	}
	if _, err := f.WriteString(Skeleton(id, title)); err != nil {
		f.Close()
		os.Remove(abs)
		return Result{}, fmt.Errorf("write %s: %w", res.Rel, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(abs)
		return Result{}, fmt.Errorf("write %s: %w", res.Rel, err)
	}
	return res, nil
}

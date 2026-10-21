// Package promote moves one inbox task file to tasks/ready and commits only the
// two task paths involved. It never edits the task contents.
package promote

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"

	"taskfactory/internal/paths"
	"taskfactory/internal/statecommit"
	"taskfactory/internal/taskvalidate"
)

var idPattern = regexp.MustCompile(`^TF-[0-9]{3}$`)

// todoPattern matches the TODO marker that "taskfactory new" leaves where prose
// is still needed.
var todoPattern = regexp.MustCompile(`\bTODO\b`)

// refuseTodo refuses a task file that still has a TODO marker, naming the first
// line. The task is not moved.
func refuseTodo(id, abs, rel string) error {
	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("read inbox task %s: %w", rel, err)
	}
	for i, line := range strings.Split(string(data), "\n") {
		if todoPattern.MatchString(line) {
			return fmt.Errorf("refusing to promote %s: %s line %d still has a TODO marker; replace every TODO with real content", id, rel, i+1)
		}
	}
	return nil
}

// otherStates are the state directories that must not hold the task being
// promoted. Promotion is refused when any of them contains a file for its ID.
var otherStates = []string{"ready", "active", "failed", "archive"}

// UsageError marks an invalid task ID argument, which the CLI reports as exit
// code 2 rather than as a refused promotion.
type UsageError struct{ msg string }

func (e UsageError) Error() string { return e.msg }

// Promote validates the single inbox task file for id against the ready
// contract, moves it to tasks/ready, validates the whole task tree and commits
// only the task paths. It returns the slash-separated ready path on success.
// Any refusal or failure restores the file to tasks/inbox and changes nothing
// else.
func Promote(projectRoot, id string) (string, error) {
	return PromoteWith(projectRoot, id, Options{})
}

// Options are the common flags of the promote command. Verbose, when non-nil,
// receives one line per Git command run and per path touched. With DryRun,
// Would receives each planned action and nothing is moved or committed.
type Options struct {
	DryRun  bool
	Verbose func(format string, args ...any)
	Would   func(format string, args ...any)
}

// trace is the verbose sink of the running call; it is nil unless Options.Verbose is set.
var trace atomic.Pointer[func(format string, args ...any)]

// setTrace installs or clears the sink; it is safe for concurrent calls.
func setTrace(f func(format string, args ...any)) {
	if f == nil {
		trace.Store(nil)
		return
	}
	trace.Store(&f)
}

func tracef(format string, args ...any) {
	if f := trace.Load(); f != nil {
		(*f)(format, args...)
	}
}

// PromoteWith is Promote with options. A dry run runs every refusal check of
// the real command; the contract checks that need the file in tasks/ready run
// on a scratch copy of the task tree outside the project, so nothing in the
// project is moved, not even briefly.
func PromoteWith(projectRoot, id string, opts Options) (string, error) {
	setTrace(opts.Verbose)
	defer setTrace(nil)
	if !idPattern.MatchString(id) {
		return "", UsageError{fmt.Sprintf("task %q is not a valid task ID (expected TF-NNN)", id)}
	}
	root, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root %s: %w", projectRoot, err)
	}
	layout := paths.For(root)
	stateRepo := statecommit.IsRepo(layout)
	// gitRoot is where task moves are committed: the project in the default
	// layout, the state repository for an external one, and "" for a plain
	// external folder, which gets renames only.
	gitRoot := ""
	if !layout.External() {
		gitRoot = root
	} else if stateRepo {
		gitRoot = layout.State
	}
	if gitRoot != "" {
		staged, err := git(gitRoot, "diff", "--cached", "--name-only")
		if err != nil {
			return "", fmt.Errorf("inspect index: %w", err)
		}
		if strings.TrimSpace(staged) != "" {
			return "", fmt.Errorf("refusing to promote %s: the index already has staged paths; unstage them first", id)
		}
	}

	name, err := findInboxFile(root, id)
	if err != nil {
		return "", err
	}
	inboxRel := layout.TaskFileRel("inbox", name)
	readyRel := layout.TaskFileRel("ready", name)
	inboxAbs := layout.TaskFile("inbox", name)
	readyAbs := layout.TaskFile("ready", name)

	info, err := os.Lstat(inboxAbs)
	if err != nil {
		return "", fmt.Errorf("inspect inbox task %s: %w", inboxRel, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("refusing to promote %s: %s is not a regular file", id, inboxRel)
	}
	if _, err := os.Lstat(readyAbs); err == nil {
		return "", fmt.Errorf("refusing to promote %s: %s already exists", id, readyRel)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect ready path %s: %w", readyRel, err)
	}

	if err := refuseTodo(id, inboxAbs, inboxRel); err != nil {
		return "", err
	}

	if opts.DryRun {
		if err := dryValidate(root, id, inboxRel, readyRel); err != nil {
			return "", err
		}
		if opts.Would != nil {
			opts.Would("move task to %s", layout.TaskDirRel("ready"))
			if gitRoot != "" {
				opts.Would("commit")
			}
		}
		return readyRel, nil
	}
	tracef("move %s to %s", inboxRel, readyRel)
	if err := os.Rename(inboxAbs, readyAbs); err != nil {
		return "", fmt.Errorf("move %s to %s: %w", inboxRel, readyRel, err)
	}
	restore := func(cause error) error {
		if err := os.Rename(readyAbs, inboxAbs); err != nil {
			return fmt.Errorf("%w; manual recovery required: restore %s to %s: %v", cause, readyRel, inboxRel, err)
		}
		return cause
	}

	if diagnostics := taskvalidate.ValidateFiles(root, []string{readyRel}); len(diagnostics) > 0 {
		return "", restore(refusal(id, diagnostics))
	}
	if diagnostics := taskvalidate.Validate(root, ""); len(diagnostics) > 0 {
		return "", restore(fmt.Errorf("task tree is invalid after promoting %s: %s", id, diagnostics[0]))
	}

	if gitRoot == "" {
		return readyRel, nil
	}
	stage := []string{readyRel}
	if _, err := git(gitRoot, "ls-files", "--error-unmatch", "--", inboxRel); err == nil {
		stage = []string{inboxRel, readyRel}
	}
	if err := commitPaths(gitRoot, id, stage); err != nil {
		// Unstage only the promoted paths so the index matches HEAD again.
		if _, resetErr := git(gitRoot, append([]string{"reset", "-q", "--"}, stage...)...); resetErr != nil {
			err = errors.Join(err, fmt.Errorf("unstage promoted paths: %w", resetErr))
		}
		return "", restore(fmt.Errorf("commit promotion of %s: %w", id, err))
	}
	return readyRel, nil
}

// dryValidate applies the same two contract checks as the real command to a
// scratch copy of the task tree in which the task has already moved.
func dryValidate(root, id, inboxRel, readyRel string) error {
	scratch, err := os.MkdirTemp("", "taskfactory-dryrun-")
	if err != nil {
		return fmt.Errorf("create scratch directory: %w", err)
	}
	defer os.RemoveAll(scratch)
	scratchLayout := paths.Layout{Project: scratch, State: scratch, TasksDir: paths.For(root).TasksDir}
	defer paths.Register(scratchLayout)()
	tracef("scratch copy of tasks/ in %s", scratch)
	if err := copyTasks(root, scratch); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(scratch, filepath.FromSlash(inboxRel)), filepath.Join(scratch, filepath.FromSlash(readyRel))); err != nil {
		return fmt.Errorf("prepare scratch copy: %w", err)
	}
	if diagnostics := taskvalidate.ValidateFiles(scratch, []string{readyRel}); len(diagnostics) > 0 {
		return refusal(id, diagnostics)
	}
	// Claim blocks of other tasks are checked against the project root, which
	// the scratch copy does not share, so the rest of the tree is validated in
	// place. The moved file's own diagnostics above include the tree-wide
	// ID and dependency checks.
	if diagnostics := taskvalidate.Validate(root, ""); len(diagnostics) > 0 {
		return fmt.Errorf("task tree is invalid after promoting %s: %s", id, diagnostics[0])
	}
	return nil
}

// copyTasks copies every regular file below root/tasks, and the configuration
// when present, into dst. It only reads root.
func copyTasks(root, dst string) error {
	lay := paths.For(root)
	state := lay.State
	for _, top := range []string{lay.TasksRel(), paths.ConfigRel()} {
		src := filepath.Join(state, filepath.FromSlash(top))
		err := filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			rel, _ := filepath.Rel(state, path)
			target := filepath.Join(dst, rel)
			if entry.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.WriteFile(target, data, 0o644)
		})
		if err != nil {
			return fmt.Errorf("prepare scratch copy: %w", err)
		}
	}
	return nil
}

// refusal formats the first contract diagnostic for a task that is not ready.
func refusal(id string, diagnostics []taskvalidate.Diagnostic) error {
	return fmt.Errorf("refusing to promote %s: %s", id, diagnostics[0])
}

// findInboxFile returns the name of the only inbox task file for id. It refuses
// when the ID is absent from the inbox, appears in another state directory, or
// matches more than one inbox file.
func findInboxFile(root, id string) (string, error) {
	prefix := id + "-"
	for _, state := range otherStates {
		entries, err := os.ReadDir(paths.For(root).TaskDir(state))
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("read %s: %w", paths.For(root).TaskDirRel(state), err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".md") {
				return "", fmt.Errorf("refusing to promote %s: it is in %s, not %s", id, paths.For(root).TaskDirRel(state), paths.For(root).TaskDirRel("inbox"))
			}
		}
	}
	entries, err := os.ReadDir(paths.For(root).TaskDir("inbox"))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", paths.For(root).TaskDirRel("inbox"), err)
	}
	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".md") {
			matches = append(matches, entry.Name())
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("refusing to promote %s: no task file exists in %s for this ID", id, paths.For(root).TaskDirRel("inbox"))
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("refusing to promote %s: multiple inbox task files match this ID", id)
	}
}

// commitPaths stages the given paths and commits only their contents with
// --only, so nothing else in the index or working tree is recorded.
func commitPaths(root, id string, stage []string) error {
	if _, err := git(root, append([]string{"add", "--all", "--"}, stage...)...); err != nil {
		return err
	}
	message := fmt.Sprintf("docs: promote %s to ready", id)
	_, err := git(root, append([]string{"commit", "--quiet", "--only", "-m", message, "--"}, stage...)...)
	return err
}

// git runs git with -C root and returns its stdout. Failures include stderr.
func git(root string, args ...string) (string, error) {
	tracef("git %s", strings.Join(args, " "))
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

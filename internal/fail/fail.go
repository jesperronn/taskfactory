// Package fail moves one claimed active task file to tasks/failed and commits
// only the two task paths involved. The file bytes are never changed, and the
// attempt evidence file is never read for writing, staged or committed.
package fail

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"taskfactory/internal/taskvalidate"
	"taskfactory/internal/verify"
)

var idPattern = regexp.MustCompile(`^TF-[0-9]{3}$`)

// otherStates are the state directories that must not hold the task being
// failed. The command is refused when any of them contains a file for its ID.
var otherStates = []string{"inbox", "ready", "failed", "archive"}

// UsageError marks invalid arguments, which the CLI reports as exit code 2
// rather than as a refused failure.
type UsageError struct{ msg string }

func (e UsageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return UsageError{fmt.Sprintf(format, args...)}
}

// Options holds the parsed flags of the fail command. ReasonSet records whether
// --reason was given, so an empty reason can be refused rather than ignored.
type Options struct {
	Outcome   string
	Reason    string
	ReasonSet bool
}

// ParseArgs parses the arguments after "fail": one task ID, a required
// --outcome and an optional --reason. Both "--flag value" and "--flag=value"
// are accepted, and flags may appear before or after the ID. Unknown, repeated
// or extra arguments are usage errors.
func ParseArgs(args []string) (string, Options, error) {
	var id string
	var opts Options
	seenOutcome := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")
		if name != "--outcome" && name != "--reason" {
			if strings.HasPrefix(arg, "-") {
				return "", Options{}, usagef("unknown argument %q", arg)
			}
			if id != "" {
				return "", Options{}, usagef("unexpected extra argument %q", arg)
			}
			id = arg
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return "", Options{}, usagef("%s requires a value", name)
			}
			i++
			value = args[i]
		}
		if name == "--outcome" {
			if seenOutcome {
				return "", Options{}, usagef("--outcome may be given only once")
			}
			seenOutcome = true
			opts.Outcome = value
			continue
		}
		if opts.ReasonSet {
			return "", Options{}, usagef("--reason may be given only once")
		}
		opts.ReasonSet = true
		opts.Reason = value
	}
	if id == "" {
		return "", Options{}, usagef("missing task ID; usage: taskfactory fail <ID> --outcome <FAILED|BLOCKED>")
	}
	if !seenOutcome {
		return "", Options{}, usagef("missing --outcome; use FAILED or BLOCKED")
	}
	if err := checkOutcome(opts.Outcome); err != nil {
		return "", Options{}, err
	}
	if opts.ReasonSet {
		if err := checkReason(opts.Reason); err != nil {
			return "", Options{}, err
		}
	}
	return id, opts, nil
}

func checkOutcome(outcome string) error {
	if outcome != "FAILED" && outcome != "BLOCKED" {
		return usagef("--outcome must be FAILED or BLOCKED, not %q", outcome)
	}
	return nil
}

func checkReason(reason string) error {
	if strings.TrimSpace(reason) == "" {
		return usagef("--reason must be non-empty text")
	}
	if strings.ContainsAny(reason, "\r\n") {
		return usagef("--reason must be single-line text")
	}
	return nil
}

// Fail moves the single active task file for id to tasks/failed when the last
// attempt evidence records opts.Outcome, or when no evidence exists and a
// reason is given. It validates the whole task tree after the move and commits
// only the task paths. It returns the slash-separated failed path on success.
// Any refusal or failure restores the file to tasks/active and changes nothing
// else.
func Fail(projectRoot, id string, opts Options) (string, error) {
	if !idPattern.MatchString(id) {
		return "", usagef("task %q is not a valid task ID (expected TF-NNN)", id)
	}
	if err := checkOutcome(opts.Outcome); err != nil {
		return "", err
	}
	if opts.ReasonSet {
		if err := checkReason(opts.Reason); err != nil {
			return "", err
		}
	}
	root, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root %s: %w", projectRoot, err)
	}
	staged, err := git(root, "diff", "--cached", "--name-only")
	if err != nil {
		return "", fmt.Errorf("inspect index: %w", err)
	}
	if strings.TrimSpace(staged) != "" {
		return "", fmt.Errorf("refusing to fail %s: the index already has staged paths; unstage them first", id)
	}

	name, err := findActiveFile(root, id)
	if err != nil {
		return "", err
	}
	activeRel := "tasks/active/" + name
	failedRel := "tasks/failed/" + name
	activeAbs := filepath.Join(root, filepath.FromSlash(activeRel))
	failedAbs := filepath.Join(root, filepath.FromSlash(failedRel))

	info, err := os.Lstat(activeAbs)
	if err != nil {
		return "", fmt.Errorf("inspect active task %s: %w", activeRel, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("refusing to fail %s: %s is not a regular file", id, activeRel)
	}
	if _, err := os.Lstat(failedAbs); err == nil {
		return "", fmt.Errorf("refusing to fail %s: %s already exists", id, failedRel)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect failed path %s: %w", failedRel, err)
	}

	if diagnostics := taskvalidate.ValidateFiles(root, []string{activeRel}); len(diagnostics) > 0 {
		return "", fmt.Errorf("refusing to fail %s: %s", id, diagnostics[0])
	}
	if err := checkEvidence(root, id, opts); err != nil {
		return "", err
	}

	if err := os.Rename(activeAbs, failedAbs); err != nil {
		return "", fmt.Errorf("move %s to %s: %w", activeRel, failedRel, err)
	}
	restore := func(cause error) error {
		if err := os.Rename(failedAbs, activeAbs); err != nil {
			return fmt.Errorf("%w; manual recovery required: restore %s to %s: %v", cause, failedRel, activeRel, err)
		}
		return cause
	}

	if diagnostics := taskvalidate.Validate(root, ""); len(diagnostics) > 0 {
		return "", restore(fmt.Errorf("task tree is invalid after failing %s: %s", id, diagnostics[0]))
	}

	paths := []string{failedRel}
	if _, err := git(root, "ls-files", "--error-unmatch", "--", activeRel); err == nil {
		paths = []string{activeRel, failedRel}
	}
	if err := commitPaths(root, id, paths, opts); err != nil {
		// Unstage only the moved paths so the index matches HEAD again.
		if _, resetErr := git(root, append([]string{"reset", "-q", "--"}, paths...)...); resetErr != nil {
			err = errors.Join(err, fmt.Errorf("unstage failed paths: %w", resetErr))
		}
		return "", restore(fmt.Errorf("commit failure of %s: %w", id, err))
	}
	return failedRel, nil
}

// checkEvidence applies the evidence rules: with records, the last outcome must
// equal opts.Outcome and --reason is refused; without records, --reason is
// required.
func checkEvidence(root, id string, opts Options) error {
	last, err := verify.LastOutcome(root, id)
	if err != nil {
		return fmt.Errorf("refusing to fail %s: %w", id, err)
	}
	if last != "" {
		if opts.ReasonSet {
			return fmt.Errorf("refusing to fail %s: --reason is refused when attempt evidence exists", id)
		}
		if last != opts.Outcome {
			return fmt.Errorf("refusing to fail %s: last evidence outcome is %s, not --outcome %s", id, last, opts.Outcome)
		}
		return nil
	}
	if !opts.ReasonSet {
		return fmt.Errorf("refusing to fail %s: no attempt evidence exists; pass --reason to record why", id)
	}
	return nil
}

// findActiveFile returns the name of the only active task file for id. It
// refuses when the ID appears in another state directory, is absent from the
// active directory, or matches more than one active file.
func findActiveFile(root, id string) (string, error) {
	prefix := id + "-"
	for _, state := range otherStates {
		entries, err := os.ReadDir(filepath.Join(root, "tasks", state))
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("read tasks/%s: %w", state, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".md") {
				return "", fmt.Errorf("refusing to fail %s: it is in tasks/%s, not tasks/active", id, state)
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "tasks", "active"))
	if err != nil {
		return "", fmt.Errorf("read tasks/active: %w", err)
	}
	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".md") {
			matches = append(matches, entry.Name())
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("refusing to fail %s: no task file exists in tasks/active for this ID", id)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("refusing to fail %s: multiple active task files match this ID", id)
	}
}

// commitPaths stages the given paths and commits only their contents with
// --only. The reason, when given, becomes the body of the commit message.
func commitPaths(root, id string, paths []string, opts Options) error {
	if _, err := git(root, append([]string{"add", "--all", "--"}, paths...)...); err != nil {
		return err
	}
	args := []string{"commit", "--quiet", "--only", "-m", fmt.Sprintf("docs: mark %s failed", id)}
	if opts.ReasonSet {
		args = append(args, "-m", opts.Reason)
	}
	_, err := git(root, append(append(args, "--"), paths...)...)
	return err
}

// git runs git with -C root and returns its stdout. Failures include stderr.
func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

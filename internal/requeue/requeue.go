// Package requeue moves one unclaimed failed task file back to tasks/ready or
// tasks/inbox and commits only the two task paths involved. The task file is
// never edited. The attempt counter is reset by renaming the old evidence file
// aside; the evidence bytes are never edited, truncated or deleted.
package requeue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"taskfactory/internal/taskvalidate"
	"taskfactory/internal/verify"
)

var idPattern = regexp.MustCompile(`^TF-[0-9]{3}$`)

// otherStates are the state directories that must not hold the task being
// requeued. The task is refused when any of them contains a file for its ID.
var otherStates = []string{"inbox", "ready", "active", "archive"}

// UsageError marks invalid arguments, which the CLI reports as exit code 2
// rather than as a refused requeue.
type UsageError struct{ msg string }

func (e UsageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return UsageError{fmt.Sprintf(format, args...)}
}

// Result describes a successful requeue. Aside is the slash-separated path the
// old evidence file was renamed to, or empty when no evidence was reset.
type Result struct {
	Path     string
	Aside    string
	Attempts int
}

// ParseArgs parses the arguments after "requeue": one task ID and an optional
// --to ready|inbox, given as "--to value" or "--to=value". It returns the ID
// and the target state, which defaults to "ready". Unknown, repeated or extra
// arguments are usage errors.
func ParseArgs(args []string) (string, string, error) {
	id, target, seenTo := "", "ready", false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")
		if name != "--to" {
			if strings.HasPrefix(arg, "-") {
				return "", "", usagef("unknown argument %q", arg)
			}
			if id != "" {
				return "", "", usagef("unexpected extra argument %q", arg)
			}
			id = arg
			continue
		}
		if seenTo {
			return "", "", usagef("--to may be given only once")
		}
		seenTo = true
		if !hasValue {
			if i+1 >= len(args) {
				return "", "", usagef("--to requires a value; use ready or inbox")
			}
			i++
			value = args[i]
		}
		if value != "ready" && value != "inbox" {
			return "", "", usagef("--to must be ready or inbox, not %q", value)
		}
		target = value
	}
	if id == "" {
		return "", "", usagef("missing task ID; usage: taskfactory requeue <ID> [--to ready|inbox]")
	}
	if !idPattern.MatchString(id) {
		return "", "", usagef("task %q is not a valid task ID (expected TF-NNN)", id)
	}
	return id, target, nil
}

// ClaimedFailed returns the sorted IDs of failed task files that still carry a
// Claim block.
func ClaimedFailed(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "tasks", "failed"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read tasks/failed: %w", err)
	}
	var ids []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || len(name) < 7 || !idPattern.MatchString(name[:6]) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "tasks", "failed", name))
		if err != nil {
			return nil, fmt.Errorf("read tasks/failed/%s: %w", name, err)
		}
		if hasClaim(data) {
			ids = append(ids, name[:6])
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func hasClaim(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimRight(line, "\r") == "## Claim" {
			return true
		}
	}
	return false
}

// Requeue moves the single failed task file for id to tasks/<target>, resets
// the attempt counter by renaming the evidence file aside, validates the task
// tree and commits only the task paths. Any refusal or failure restores the
// file and the evidence name and changes nothing else.
func Requeue(projectRoot, id, target string) (Result, error) {
	if !idPattern.MatchString(id) {
		return Result{}, usagef("task %q is not a valid task ID (expected TF-NNN)", id)
	}
	if target != "ready" && target != "inbox" {
		return Result{}, usagef("--to must be ready or inbox, not %q", target)
	}
	root, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return Result{}, fmt.Errorf("resolve project root %s: %w", projectRoot, err)
	}
	staged, err := git(root, "diff", "--cached", "--name-only")
	if err != nil {
		return Result{}, fmt.Errorf("inspect index: %w", err)
	}
	if strings.TrimSpace(staged) != "" {
		return Result{}, fmt.Errorf("refusing to requeue %s: the index already has staged paths; unstage them first", id)
	}

	name, err := findFailedFile(root, id)
	if err != nil {
		return Result{}, err
	}
	failedRel := "tasks/failed/" + name
	targetRel := "tasks/" + target + "/" + name
	failedAbs := filepath.Join(root, filepath.FromSlash(failedRel))
	targetAbs := filepath.Join(root, filepath.FromSlash(targetRel))

	info, err := os.Lstat(failedAbs)
	if err != nil {
		return Result{}, fmt.Errorf("inspect failed task %s: %w", failedRel, err)
	}
	if !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("refusing to requeue %s: %s is not a regular file", id, failedRel)
	}
	data, err := os.ReadFile(failedAbs)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", failedRel, err)
	}
	if hasClaim(data) {
		return Result{}, claimedRefusal(root, id)
	}
	if _, err := os.Lstat(targetAbs); err == nil {
		return Result{}, fmt.Errorf("refusing to requeue %s: %s already exists", id, targetRel)
	} else if !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("inspect %s: %w", targetRel, err)
	}

	evidenceRel, asideRel, attempts, err := planEvidence(root, id)
	if err != nil {
		return Result{}, err
	}

	paths := []string{targetRel}
	if _, err := git(root, "ls-files", "--error-unmatch", "--", failedRel); err == nil {
		paths = []string{failedRel, targetRel}
	}

	if err := os.Rename(failedAbs, targetAbs); err != nil {
		return Result{}, fmt.Errorf("move %s to %s: %w", failedRel, targetRel, err)
	}
	restore := func(cause error) error {
		if err := os.Rename(targetAbs, failedAbs); err != nil {
			return fmt.Errorf("%w; manual recovery required: restore %s to %s: %v", cause, targetRel, failedRel, err)
		}
		return cause
	}
	if target == "ready" {
		if diagnostics := taskvalidate.ValidateFiles(root, []string{targetRel}); len(diagnostics) > 0 {
			return Result{}, restore(fmt.Errorf("refusing to requeue %s: %s", id, diagnostics[0]))
		}
	}
	if diagnostics := taskvalidate.Validate(root, ""); len(diagnostics) > 0 {
		return Result{}, restore(fmt.Errorf("task tree is invalid after requeueing %s: %s", id, diagnostics[0]))
	}

	if asideRel != "" {
		evidenceAbs := filepath.Join(root, filepath.FromSlash(evidenceRel))
		asideAbs := filepath.Join(root, filepath.FromSlash(asideRel))
		if err := os.Rename(evidenceAbs, asideAbs); err != nil {
			return Result{}, restore(fmt.Errorf("rename %s aside: %w", evidenceRel, err))
		}
		restoreAll := restore
		restore = func(cause error) error {
			if err := os.Rename(asideAbs, evidenceAbs); err != nil {
				cause = fmt.Errorf("%w; manual recovery required: restore %s to %s: %v", cause, asideRel, evidenceRel, err)
			}
			return restoreAll(cause)
		}
	}

	if err := commitPaths(root, id, target, paths, asideRel, attempts); err != nil {
		// Unstage only the moved paths so the index matches HEAD again.
		if _, resetErr := git(root, append([]string{"reset", "-q", "--"}, paths...)...); resetErr != nil {
			err = errors.Join(err, fmt.Errorf("unstage requeued paths: %w", resetErr))
		}
		return Result{}, restore(fmt.Errorf("commit requeue of %s: %w", id, err))
	}
	return Result{Path: targetRel, Aside: asideRel, Attempts: attempts}, nil
}

// claimedRefusal builds the human-decision refusal listing every claimed failed
// task ID.
func claimedRefusal(root, id string) error {
	ids, err := ClaimedFailed(root)
	list := strings.Join(ids, ", ")
	if err != nil || list == "" {
		list = id
	}
	return fmt.Errorf("refusing to requeue %s: it still has a Claim block, so it is flagged for a human decision; claimed failed tasks: %s", id, list)
}

// planEvidence returns the evidence path, the aside path and the highest
// attempt number when the task has a non-empty evidence file. With no evidence
// both paths are empty. It refuses an invalid evidence file or an aside path
// that already exists.
func planEvidence(root, id string) (evidenceRel, asideRel string, attempts int, err error) {
	evidenceRel = ".taskfactory/evidence/" + id + ".jsonl"
	evidenceAbs := filepath.Join(root, filepath.FromSlash(evidenceRel))
	data, err := os.ReadFile(evidenceAbs)
	if os.IsNotExist(err) {
		return "", "", 0, nil
	}
	if err != nil {
		return "", "", 0, fmt.Errorf("refusing to requeue %s: read evidence: %w", id, err)
	}
	if len(data) == 0 {
		return "", "", 0, nil
	}
	if _, err := verify.LastOutcome(root, id); err != nil {
		return "", "", 0, fmt.Errorf("refusing to requeue %s: %w", id, err)
	}
	lines := bytes.Split(data[:len(data)-1], []byte{'\n'})
	var last struct {
		Attempt int `json:"attempt"`
	}
	if err := json.Unmarshal(lines[len(lines)-1], &last); err != nil || last.Attempt < 1 {
		return "", "", 0, fmt.Errorf("refusing to requeue %s: evidence has an unreadable last attempt number", id)
	}
	attempts = last.Attempt
	asideRel = fmt.Sprintf(".taskfactory/evidence/%s.attempts-%d.jsonl", id, attempts)
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(asideRel))); err == nil {
		return "", "", 0, fmt.Errorf("refusing to requeue %s: %s already exists; it is never overwritten", id, asideRel)
	} else if !os.IsNotExist(err) {
		return "", "", 0, fmt.Errorf("inspect %s: %w", asideRel, err)
	}
	return evidenceRel, asideRel, attempts, nil
}

// findFailedFile returns the name of the only failed task file for id. It
// refuses when the ID appears in another state directory, is absent from
// tasks/failed, or matches more than one failed file.
func findFailedFile(root, id string) (string, error) {
	prefix := id + "-"
	for _, state := range otherStates {
		entries, err := os.ReadDir(filepath.Join(root, "tasks", state))
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("read tasks/%s: %w", state, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".md") {
				return "", fmt.Errorf("refusing to requeue %s: it is in tasks/%s, not tasks/failed", id, state)
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "tasks", "failed"))
	if err != nil {
		return "", fmt.Errorf("read tasks/failed: %w", err)
	}
	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".md") {
			matches = append(matches, entry.Name())
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("refusing to requeue %s: no task file exists in tasks/failed for this ID", id)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("refusing to requeue %s: multiple failed task files match this ID", id)
	}
}

// commitPaths stages the given paths and commits only their contents with
// --only. When the counter was reset, the body names the aside file.
func commitPaths(root, id, target string, paths []string, asideRel string, attempts int) error {
	if _, err := git(root, append([]string{"add", "--all", "--"}, paths...)...); err != nil {
		return err
	}
	args := []string{"commit", "--quiet", "--only", "-m", fmt.Sprintf("docs: requeue %s to %s", id, target)}
	if asideRel != "" {
		args = append(args, "-m", fmt.Sprintf("Attempt counter reset: the evidence file with %d attempt(s) was renamed to %s. The next attempt starts at 1.", attempts, asideRel))
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

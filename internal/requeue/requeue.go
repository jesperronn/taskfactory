// Package requeue moves one failed task file back to tasks/ready or
// tasks/inbox and commits only the two task paths involved. The task file is
// never edited, except that --discard-claim removes its Claim block. The attempt
// counter is reset by renaming the old evidence file aside; the evidence bytes
// are never edited, truncated or deleted. After the commit it removes the task's
// old worktree and branch when that is safe (see cleanup.go).
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

// Options holds the optional requeue flags. DiscardClaim removes the Claim
// block from a claimed failed task and records it in the commit message.
type Options struct {
	DiscardClaim bool
	// DryRun makes RequeueWith stop before the first write and report each
	// planned action to Would. Verbose, when non-nil, receives one line per
	// Git command run and per path touched.
	DryRun  bool
	Verbose func(format string, args ...any)
	Would   func(format string, args ...any)
}

// trace is the verbose sink of the running call; it is nil unless Verbose is set.
var trace func(format string, args ...any)

func tracef(format string, args ...any) {
	if trace != nil {
		trace(format, args...)
	}
}

// Result describes a successful requeue. Aside is the slash-separated path the
// old evidence file was renamed to, or empty when no evidence was reset.
// Discarded is the Claim block that was removed, or empty when none was.
type Result struct {
	Path      string
	Aside     string
	Attempts  int
	Discarded string
	// RemovedWorktree and RemovedBranch name what the safe cleanup removed
	// after the commit; each is empty when there was nothing to remove.
	RemovedWorktree string
	RemovedBranch   string
}

// ParseArgs parses the arguments after "requeue" like ParseOptions and drops
// the options. It returns the ID and the target state.
func ParseArgs(args []string) (string, string, error) {
	id, target, _, err := ParseOptions(args)
	return id, target, err
}

// ParseOptions parses the arguments after "requeue": one task ID, an optional
// --to ready|inbox, given as "--to value" or "--to=value", and an optional
// --discard-claim. It returns the ID, the target state, which defaults to
// "ready", and the options. Unknown, repeated or extra arguments and a value
// given to --discard-claim are usage errors.
func ParseOptions(args []string) (string, string, Options, error) {
	id, target, seenTo := "", "ready", false
	var opts Options
	seenDiscard := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")
		if name == "--discard-claim" {
			if hasValue {
				return "", "", opts, usagef("--discard-claim takes no value, got %q", arg)
			}
			if seenDiscard {
				return "", "", opts, usagef("--discard-claim may be given only once")
			}
			seenDiscard = true
			opts.DiscardClaim = true
			continue
		}
		if name != "--to" {
			if strings.HasPrefix(arg, "-") {
				return "", "", opts, usagef("unknown argument %q", arg)
			}
			if id != "" {
				return "", "", opts, usagef("unexpected extra argument %q", arg)
			}
			id = arg
			continue
		}
		if seenTo {
			return "", "", opts, usagef("--to may be given only once")
		}
		seenTo = true
		if !hasValue {
			if i+1 >= len(args) {
				return "", "", opts, usagef("--to requires a value; use ready or inbox")
			}
			i++
			value = args[i]
		}
		if value != "ready" && value != "inbox" {
			return "", "", opts, usagef("--to must be ready or inbox, not %q", value)
		}
		target = value
	}
	if id == "" {
		return "", "", opts, usagef("missing task ID; usage: taskfactory requeue <ID> [--to ready|inbox] [--discard-claim]")
	}
	if !idPattern.MatchString(id) {
		return "", "", opts, usagef("task %q is not a valid task ID (expected TF-NNN)", id)
	}
	return id, target, opts, nil
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
	return RequeueWith(projectRoot, id, target, Options{})
}

// RequeueWith is Requeue with options. With DiscardClaim, a Claim block in the
// failed task is removed from the moved file and recorded in the commit message
// body. Whenever the old worktree and branch are clean and merged into main they
// are removed after the commit; otherwise requeue refuses before any change.
// Without the flag, a claimed task is refused as before.
func RequeueWith(projectRoot, id, target string, opts Options) (Result, error) {
	trace = opts.Verbose
	defer func() { trace = nil }()
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
	newData, discarded := data, ""
	if hasClaim(data) {
		if !opts.DiscardClaim {
			return Result{}, claimedRefusal(root, id)
		}
		newData, discarded = stripClaim(data)
		if hasClaim(newData) {
			return Result{}, fmt.Errorf("refusing to requeue %s: the Claim block could not be removed cleanly", id)
		}
	}
	// Safety checks for the old worktree and branch run before any change.
	plan, err := planCleanup(root, id, claimBlockOf(data, hasClaim(data)))
	if err != nil {
		return Result{}, err
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

	if opts.DryRun {
		return dryRun(root, id, target, targetRel, plan, newData, discarded, asideRel, attempts, opts)
	}
	tracef("move %s to %s", failedRel, targetRel)
	if err := os.Rename(failedAbs, targetAbs); err != nil {
		return Result{}, fmt.Errorf("move %s to %s: %w", failedRel, targetRel, err)
	}
	restore := func(cause error) error {
		if err := os.Rename(targetAbs, failedAbs); err != nil {
			return fmt.Errorf("%w; manual recovery required: restore %s to %s: %v", cause, targetRel, failedRel, err)
		}
		if discarded != "" {
			if err := os.WriteFile(failedAbs, data, info.Mode().Perm()); err != nil {
				return fmt.Errorf("%w; manual recovery required: rewrite the Claim block of %s: %v", cause, failedRel, err)
			}
		}
		return cause
	}
	if discarded != "" {
		if err := os.WriteFile(targetAbs, newData, info.Mode().Perm()); err != nil {
			return Result{}, restore(fmt.Errorf("remove the Claim block from %s: %w", targetRel, err))
		}
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

	if err := commitPaths(root, id, target, paths, asideRel, attempts, discarded); err != nil {
		// Unstage only the moved paths so the index matches HEAD again.
		if _, resetErr := git(root, append([]string{"reset", "-q", "--"}, paths...)...); resetErr != nil {
			err = errors.Join(err, fmt.Errorf("unstage requeued paths: %w", resetErr))
		}
		return Result{}, restore(fmt.Errorf("commit requeue of %s: %w", id, err))
	}
	res := Result{Path: targetRel, Aside: asideRel, Attempts: attempts, Discarded: discarded}
	// The cleanup runs last, after the requeue commit succeeded.
	removedWorktree, removedBranch, err := plan.run(root)
	res.RemovedWorktree, res.RemovedBranch = removedWorktree, removedBranch
	if err != nil {
		return res, cleanupFailure(id, targetRel, plan, removedWorktree, err)
	}
	return res, nil
}

// dryRun runs the contract checks of the real command on a scratch copy of the
// task tree outside the project, in which the task has already moved, and then
// reports each action the real command would take. It writes nothing in the
// project.
func dryRun(root, id, target, targetRel string, plan cleanupPlan, newData []byte, discarded, asideRel string, attempts int, opts Options) (Result, error) {
	scratch, err := os.MkdirTemp("", "taskfactory-dryrun-")
	if err != nil {
		return Result{}, fmt.Errorf("create scratch directory: %w", err)
	}
	defer os.RemoveAll(scratch)
	tracef("scratch copy of tasks/ in %s", scratch)
	if err := copyTasks(root, scratch); err != nil {
		return Result{}, err
	}
	name := filepath.Base(targetRel)
	scratchTarget := filepath.Join(scratch, filepath.FromSlash(targetRel))
	if err := os.Remove(filepath.Join(scratch, "tasks", "failed", name)); err != nil {
		return Result{}, fmt.Errorf("prepare scratch copy: %w", err)
	}
	if err := os.WriteFile(scratchTarget, newData, 0o644); err != nil {
		return Result{}, fmt.Errorf("prepare scratch copy: %w", err)
	}
	if target == "ready" {
		if diagnostics := taskvalidate.ValidateFiles(scratch, []string{targetRel}); len(diagnostics) > 0 {
			return Result{}, fmt.Errorf("refusing to requeue %s: %s", id, diagnostics[0])
		}
	}
	// Claim blocks of other tasks are checked against the project root, which
	// the scratch copy does not share, so the tree is validated in place.
	if diagnostics := taskvalidate.Validate(root, ""); len(diagnostics) > 0 {
		return Result{}, fmt.Errorf("task tree is invalid after requeueing %s: %s", id, diagnostics[0])
	}
	if opts.Would != nil {
		opts.Would("move task to tasks/%s", target)
		if discarded != "" {
			opts.Would("remove the Claim block")
		}
		if asideRel != "" {
			opts.Would("rename evidence aside to %s", asideRel)
		}
		opts.Would("commit")
		if plan.hasDir {
			opts.Would("remove worktree %s", plan.worktree)
		}
		if plan.hasBranch {
			opts.Would("remove branch %s", plan.branch)
		}
	}
	return Result{Path: targetRel, Aside: asideRel, Attempts: attempts, Discarded: discarded}, nil
}

// copyTasks copies every regular file below root/tasks, and the configuration
// when present, into dst. It only reads root.
func copyTasks(root, dst string) error {
	for _, top := range []string{"tasks", ".taskfactory/config.toml"} {
		src := filepath.Join(root, filepath.FromSlash(top))
		err := filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			rel, _ := filepath.Rel(root, path)
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

// claimBlockOf returns the Claim block text of data, or "" when it has none.
func claimBlockOf(data []byte, claimed bool) string {
	if !claimed {
		return ""
	}
	_, block := stripClaim(data)
	return block
}

// claimedRefusal builds the human-decision refusal listing every claimed failed
// task ID.
func claimedRefusal(root, id string) error {
	ids, err := ClaimedFailed(root)
	list := strings.Join(ids, ", ")
	if err != nil || list == "" {
		list = id
	}
	return fmt.Errorf("refusing to requeue %s: it still has a Claim block, so it is flagged for a human decision; claimed failed tasks: %s; to decide, rerun with --discard-claim, which removes the Claim block and records it in the commit message", id, list)
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
func commitPaths(root, id, target string, paths []string, asideRel string, attempts int, discarded string) error {
	if _, err := git(root, append([]string{"add", "--all", "--"}, paths...)...); err != nil {
		return err
	}
	args := []string{"commit", "--quiet", "--only", "-m", fmt.Sprintf("docs: requeue %s to %s", id, target)}
	if asideRel != "" {
		args = append(args, "-m", fmt.Sprintf("Attempt counter reset: the evidence file with %d attempt(s) was renamed to %s. The next attempt starts at 1.", attempts, asideRel))
	}
	if discarded != "" {
		args = append(args, "-m", "Discarded claim (removed from the task file; the worktree and branch were not touched):\n\n"+discarded)
	}
	_, err := git(root, append(append(args, "--"), paths...)...)
	return err
}

// stripClaim removes every "## Claim" section from data: the heading line, the
// lines up to the next "## " heading or the end of the file, and the blank
// lines before the heading. The rest of the file is kept byte for byte. It
// returns the new bytes and the removed block, trimmed of surrounding blank
// lines and with the lines exactly as they were.
func stripClaim(data []byte) ([]byte, string) {
	lines := strings.SplitAfter(string(data), "\n")
	var kept, removed []string
	for i := 0; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") != "## Claim" {
			kept = append(kept, lines[i])
			continue
		}
		for len(kept) > 0 && strings.TrimRight(kept[len(kept)-1], "\r\n") == "" {
			kept = kept[:len(kept)-1]
		}
		start := i
		i++
		for i < len(lines) && !strings.HasPrefix(lines[i], "## ") {
			i++
		}
		removed = append(removed, strings.TrimSpace(strings.Join(lines[start:i], "")))
		if i < len(lines) {
			kept = append(kept, "\n")
		}
		i--
	}
	return []byte(strings.Join(kept, "")), strings.Join(removed, "\n\n")
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

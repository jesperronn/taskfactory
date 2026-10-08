// Package integrate serializes verified task integration onto local main.
package integrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"taskfactory/internal/config"
	"taskfactory/internal/taskvalidate"
)

var idPattern = regexp.MustCompile(`^TF-[0-9]{3}$`)
var oidPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

type workerRecord struct {
	TaskID       string        `json:"task_id"`
	Attempt      int           `json:"attempt"`
	RecordedAt   string        `json:"recorded_at"`
	Outcome      string        `json:"outcome"`
	Branch       string        `json:"branch"`
	Base         string        `json:"base_commit"`
	Result       string        `json:"result_commit"`
	ChangedFiles []string      `json:"changed_files"`
	Checks       []workerCheck `json:"checks"`
	Note         string        `json:"note"`
}
type workerCheck struct {
	Source    string `json:"source"`
	Criterion string `json:"criterion"`
	Command   string `json:"command"`
	ExitCode  *int   `json:"exit_code"`
	Output    string `json:"output"`
	Error     string `json:"error"`
}
type attempt struct {
	TaskID       string `json:"task_id"`
	Attempt      int    `json:"attempt"`
	RecordedAt   string `json:"recorded_at"`
	Outcome      string `json:"outcome"`
	Stage        string `json:"stage"`
	Branch       string `json:"branch"`
	WorkerResult string `json:"worker_result_commit"`
	MainBefore   string `json:"main_before"`
	Verified     string `json:"verified_commit"`
	MainAfter    string `json:"main_after"`
	Command      string `json:"command"`
	ExitCode     *int   `json:"exit_code"`
	Output       string `json:"output"`
	Error        string `json:"error"`
	Note         string `json:"note"`
}
type failure struct {
	outcome, stage, command, output, note string
	code                                  *int
	err                                   error
}

// Run integrates one active task. All state inspection and changes occur under
// the repository-local integration lock.
func Run(root, id string) (retErr error) {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("task %q is not a valid task ID (expected TF-NNN)", id)
	}
	root, retErr = filepath.EvalSymlinks(root)
	if retErr != nil {
		return retErr
	}
	lockPath := filepath.Join(root, ".taskfactory/integration.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return err
	}
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lf.Close()
	if err = syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
	entry := attempt{TaskID: id, Attempt: nextAttempt(root, id), RecordedAt: time.Now().UTC().Format(time.RFC3339), Outcome: "FAILED", Stage: "eligibility", ExitCode: nil}
	var fail *failure
	appended := false
	defer func() {
		if appended {
			return
		}
		if fail != nil {
			entry.Outcome = fail.outcome
			entry.Stage = fail.stage
			entry.Command = fail.command
			entry.Output = fail.output
			entry.Error = ""
			entry.Note = fail.note
			entry.ExitCode = fail.code
			if fail.err != nil {
				entry.Error = fail.err.Error()
			}
		}
		if entry.MainAfter == "" {
			if v, e := git(root, "rev-parse", "--verify", "refs/heads/main^{commit}"); e == nil {
				entry.MainAfter = v
			}
		}
		if e := appendRecord(root, id, entry); e != nil {
			retErr = errors.Join(retErr, fmt.Errorf("append integration evidence: %w", e))
		}
	}()
	if _, e := os.Lstat(filepath.Join(root, ".taskfactory/integration-stop.json")); e == nil {
		stopErr := describeStop(root)
		fail = &failure{outcome: "BLOCKED", stage: "eligibility", note: stopErr.Error()}
		return stopErr
	} else if !os.IsNotExist(e) {
		fail = &failure{outcome: "BLOCKED", stage: "eligibility", err: e}
		return e
	}
	if _, e := os.Lstat(filepath.Join(root, ".taskfactory/integration-stop.json.tmp")); e == nil {
		fail = &failure{outcome: "BLOCKED", stage: "eligibility", note: "integration stopped by .taskfactory/integration-stop.json.tmp; inspect and repair main"}
		return fmt.Errorf("integration stopped: inspect .taskfactory/integration-stop.json.tmp and repair main")
	}
	cfg, e := config.Load(root)
	if e != nil {
		fail = &failure{outcome: "BLOCKED", stage: "eligibility", err: e}
		return e
	}
	if e = cleanConfig(root); e != nil {
		fail = &failure{outcome: "FAILED", stage: "eligibility", err: e}
		return e
	}
	branch, base, candidate, taskPath, e := candidateDetails(root, id)
	if e != nil {
		fail = &failure{outcome: "FAILED", stage: "eligibility", err: e}
		return e
	}
	entry.Branch = branch
	wr, e := latestWorker(root, id, branch, base)
	if e != nil {
		fail = &failure{outcome: "FAILED", stage: "eligibility", err: e}
		return e
	}
	entry.WorkerResult = wr.Result
	actual, e := git(candidate, "rev-parse", "HEAD")
	if e != nil || actual != wr.Result {
		fail = &failure{outcome: "FAILED", stage: "eligibility", err: fmt.Errorf("task %s candidate HEAD does not match latest worker PASS result commit", id)}
		return fail.err
	}
	if e = cleanTree(candidate); e != nil {
		fail = &failure{outcome: "FAILED", stage: "eligibility", err: fmt.Errorf("task %s candidate is not clean: %w", id, e)}
		return fail.err
	}
	main, e := git(root, "rev-parse", "--verify", "refs/heads/main^{commit}")
	if e != nil {
		fail = &failure{outcome: "BLOCKED", stage: "eligibility", err: fmt.Errorf("local refs/heads/main is unavailable: %w", e)}
		return fail.err
	}
	entry.MainBefore = main
	if branchName, _ := git(root, "branch", "--show-current"); branchName != "main" {
		fail = &failure{outcome: "BLOCKED", stage: "eligibility", err: fmt.Errorf("repository worktree must have local main checked out (currently %q)", branchName)}
		return fail.err
	}
	if e = cleanMainRuntime(root, id, cfg.Git.WorktreeRoot); e != nil {
		fail = &failure{outcome: "FAILED", stage: "eligibility", err: e}
		return e
	}
	if e = checkRuntimeInventory(root, cfg.Git.WorktreeRoot); e != nil {
		fail = &failure{outcome: "FAILED", stage: "eligibility", err: e}
		return e
	}
	for {
		if output, rebaseErr := gitOutputErr(candidate, "rebase", main); rebaseErr != nil {
			_, _ = git(candidate, "rebase", "--abort")
			fail = &failure{outcome: "FAILED", stage: "rebase", output: output, err: rebaseErr}
			return fmt.Errorf("rebase task %s onto main: %w", id, rebaseErr)
		}
		verified, e := git(candidate, "rev-parse", "HEAD")
		if e != nil {
			fail = &failure{outcome: "FAILED", stage: "rebase", err: e}
			return e
		}
		entry.Verified = verified
		for _, command := range cfg.Verification.Integration {
			out, code, startErr := runCommand(candidate, command)
			if startErr != nil {
				fail = &failure{outcome: "BLOCKED", stage: "integration_check", command: command, output: out, err: startErr}
				return fmt.Errorf("start integration check %q: %w", command, startErr)
			}
			if *code != 0 {
				fail = &failure{outcome: "FAILED", stage: "integration_check", command: command, output: out, code: code}
				return fmt.Errorf("integration check %q failed with exit code %d", command, *code)
			}
		}
		if e = cleanTree(candidate); e != nil {
			fail = &failure{outcome: "FAILED", stage: "integration_check", err: fmt.Errorf("integration checks changed candidate worktree: %w", e)}
			return fail.err
		}
		if e = cleanConfig(root); e != nil {
			fail = &failure{outcome: "FAILED", stage: "merge", err: e}
			return e
		}
		if e = cleanMainRuntime(root, id, cfg.Git.WorktreeRoot); e != nil {
			fail = &failure{outcome: "FAILED", stage: "merge", err: e}
			return e
		}
		if e = checkRuntimeInventory(root, cfg.Git.WorktreeRoot); e != nil {
			fail = &failure{outcome: "FAILED", stage: "merge", err: e}
			return e
		}
		current, e := git(root, "rev-parse", "--verify", "refs/heads/main^{commit}")
		if e != nil {
			fail = &failure{outcome: "BLOCKED", stage: "merge", err: e}
			return e
		}
		if current != main {
			main = current
			continue
		}
		break
	}
	if e = cleanConfig(root); e != nil {
		fail = &failure{outcome: "FAILED", stage: "merge", err: e}
		return e
	}
	if out, err := gitOutputErr(root, "merge", "--ff-only", entry.Verified); err != nil {
		fail = &failure{outcome: "FAILED", stage: "merge", output: out, err: err}
		return fmt.Errorf("fast-forward local main: %w", err)
	}
	entry.MainAfter, _ = git(root, "rev-parse", "HEAD")
	for _, command := range cfg.Verification.Main {
		out, code, startErr := runCommand(root, command)
		if startErr != nil || *code != 0 {
			if startErr != nil {
				fail = &failure{outcome: "BLOCKED", stage: "main_check", command: command, output: out, err: startErr}
			} else {
				fail = &failure{outcome: "FAILED", stage: "main_check", command: command, output: out, code: code}
			}
			if cfg.Integration.StopOnMainFailure {
				if se := writeStop(root, entry.MainAfter, command, code, out, startErr); se != nil {
					fail.note = "main advanced; integration stop state could not be persisted: " + se.Error()
				}
			}
			return fmt.Errorf("main check %q failed; main remains advanced and task remains active", command)
		}
	}
	if e = cleanConfig(root); e != nil {
		fail = &failure{outcome: "FAILED", stage: "archive", err: fmt.Errorf("main advanced; configuration changed before archive: %w", e)}
		return fail.err
	}
	archive := filepath.Join(root, "tasks/archive", filepath.Base(taskPath))
	if _, e = os.Lstat(archive); e == nil {
		fail = &failure{outcome: "FAILED", stage: "archive", err: fmt.Errorf("archive task path already exists: %s", archive)}
		return fail.err
	}
	if e = os.Rename(taskPath, archive); e != nil {
		fail = &failure{outcome: "FAILED", stage: "archive", err: e}
		return fmt.Errorf("move task to archive: %w", e)
	}
	archiveRel := filepath.ToSlash(filepath.Join("tasks/archive", filepath.Base(taskPath)))
	activeRel := filepath.ToSlash(filepath.Join("tasks/active", filepath.Base(taskPath)))
	stagePaths := []string{archiveRel}
	if _, trackedErr := git(root, "ls-files", "--error-unmatch", activeRel); trackedErr == nil {
		stagePaths = append(stagePaths, activeRel)
	} else {
		// Claims are deliberately uncommitted. When active is untracked, the
		// committed lifecycle transition is the tracked ready deletion plus the
		// archive addition; the claimed active file is operational state.
		readyRel := filepath.ToSlash(filepath.Join("tasks/ready", filepath.Base(taskPath)))
		if _, readyErr := git(root, "ls-files", "--error-unmatch", readyRel); readyErr == nil {
			stagePaths = append(stagePaths, readyRel)
		} else {
			_ = os.Rename(archive, taskPath)
			fail = &failure{outcome: "FAILED", stage: "archive", err: fmt.Errorf("cannot identify tracked lifecycle deletion for %s", id)}
			return fail.err
		}
	}
	addArgs := append([]string{"add", "-A", "--"}, stagePaths...)
	if out, err := gitOutputErr(root, addArgs...); err != nil {
		_ = os.Rename(archive, taskPath)
		fail = &failure{outcome: "FAILED", stage: "archive", output: out, err: err}
		return err
	}
	if out, err := gitOutputErr(root, "commit", "-m", "chore(tasks): archive integrated "+id); err != nil {
		resetArgs := append([]string{"reset", "--"}, stagePaths...)
		_, _ = git(root, resetArgs...)
		_ = os.Rename(archive, taskPath)
		fail = &failure{outcome: "FAILED", stage: "archive", output: out, err: err}
		return fmt.Errorf("commit archive transition: %w", err)
	}
	entry.Outcome = "PASS"
	entry.Stage = "archive"
	entry.MainAfter, _ = git(root, "rev-parse", "HEAD")
	entry.Command = ""
	entry.ExitCode = nil
	entry.Output = ""
	entry.Error = ""
	entry.Note = "Integration checks passed; lifecycle archive commit followed the feature commit."
	appended = true
	if e = appendRecord(root, id, entry); e != nil {
		return fmt.Errorf("archive committed but append PASS integration evidence failed: %w", e)
	}
	// The deferred failure record is suppressed after the explicit PASS append.
	fail = nil
	retErr = nil
	return nil
}

func candidateDetails(root, id string) (branch, base, worktree, path string, err error) {
	dir := filepath.Join(root, "tasks/active")
	es, e := os.ReadDir(dir)
	if e != nil {
		return "", "", "", "", e
	}
	matches := []string{}
	for _, x := range es {
		if !x.IsDir() && strings.HasPrefix(x.Name(), id+"-") && strings.HasSuffix(x.Name(), ".md") {
			matches = append(matches, filepath.Join(dir, x.Name()))
		}
	}
	if len(matches) != 1 {
		return "", "", "", "", fmt.Errorf("task %s must have exactly one active task file", id)
	}
	path = matches[0]
	if ds := taskvalidate.Validate(root, filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))); len(ds) > 0 {
		return "", "", "", "", fmt.Errorf("active task %s is malformed: %s", id, ds[0])
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", "", "", "", e
	}
	section := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "## Claim" {
			section = true
			continue
		}
		if strings.HasPrefix(line, "## ") {
			section = false
		}
		if section {
			for _, key := range []string{"Branch", "Base commit", "Worktree"} {
				if strings.HasPrefix(line, key+":") {
					v := strings.TrimSpace(strings.TrimPrefix(line, key+":"))
					switch key {
					case "Branch":
						branch = v
					case "Base commit":
						base = v
					case "Worktree":
						worktree = v
					}
				}
			}
		}
	}
	if branch == "" || !oidPattern.MatchString(base) || worktree == "" {
		return "", "", "", "", fmt.Errorf("task %s has incomplete Claim metadata", id)
	}
	if !filepath.IsAbs(worktree) {
		worktree = filepath.Join(root, worktree)
	}
	abs, e := filepath.Abs(worktree)
	if e != nil {
		return "", "", "", "", e
	}
	worktree = abs
	got, e := git(worktree, "branch", "--show-current")
	if e != nil || got != branch {
		return "", "", "", "", fmt.Errorf("task %s claimed worktree branch does not match %q", id, branch)
	}
	listing, e := git(root, "worktree", "list", "--porcelain")
	registered := false
	for _, block := range strings.Split(listing, "\n\n") {
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "worktree ") {
				listed := strings.TrimPrefix(line, "worktree ")
				resolvedListed, listedErr := filepath.EvalSymlinks(listed)
				resolvedWorktree, worktreeErr := filepath.EvalSymlinks(worktree)
				if listedErr == nil && worktreeErr == nil && filepath.Clean(resolvedListed) == filepath.Clean(resolvedWorktree) {
					registered = true
				}
			}
		}
	}
	if e != nil || !registered {
		return "", "", "", "", fmt.Errorf("task %s worktree is not registered at %s", id, worktree)
	}
	return branch, base, worktree, path, nil
}

func latestWorker(root, id, branch, base string) (workerRecord, error) {
	var last workerRecord
	p := filepath.Join(root, ".taskfactory/evidence", id+".jsonl")
	b, e := os.ReadFile(p)
	if e != nil {
		return last, fmt.Errorf("worker evidence for %s: %w", id, e)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		return last, fmt.Errorf("worker evidence for %s is incomplete", id)
	}
	lines := bytes.Split(b[:len(b)-1], []byte{'\n'})
	for i, line := range lines {
		var r workerRecord
		var raw map[string]json.RawMessage
		if e = json.Unmarshal(line, &raw); e != nil || !exactKeys(raw, []string{"task_id", "attempt", "recorded_at", "outcome", "branch", "base_commit", "result_commit", "changed_files", "checks", "note"}) {
			return last, fmt.Errorf("worker evidence for %s line %d is malformed", id, i+1)
		}
		if e = json.Unmarshal(line, &r); e != nil || r.TaskID != id || r.Attempt != i+1 || !validUTC(r.RecordedAt) || (r.Outcome != "PASS" && r.Outcome != "FAILED" && r.Outcome != "BLOCKED") || strings.TrimSpace(r.Branch) == "" || !oidPattern.MatchString(r.Base) || (r.Result != "" && !oidPattern.MatchString(r.Result)) || len(r.Checks) == 0 {
			return last, fmt.Errorf("worker evidence for %s line %d is invalid", id, i+1)
		}
		var checks []map[string]json.RawMessage
		if json.Unmarshal(raw["checks"], &checks) != nil {
			return last, fmt.Errorf("worker evidence for %s line %d has invalid checks", id, i+1)
		}
		for n, c := range checks {
			if !exactKeys(c, []string{"source", "criterion", "command", "exit_code", "output", "error"}) || r.Checks[n].Command == "" || ((r.Checks[n].ExitCode == nil) != (r.Checks[n].Error != "")) {
				return last, fmt.Errorf("worker evidence for %s line %d has invalid check result", id, i+1)
			}
		}
		if r.Outcome == "PASS" {
			if r.Result == "" {
				return last, fmt.Errorf("worker evidence for %s line %d PASS has no result commit", id, i+1)
			}
			for _, c := range r.Checks {
				if c.ExitCode == nil || *c.ExitCode != 0 {
					return last, fmt.Errorf("worker evidence for %s line %d PASS has failed checks", id, i+1)
				}
			}
		}
		last = r
	}
	if last.Outcome != "PASS" || last.Branch != branch || last.Base != base || !oidPattern.MatchString(last.Result) {
		return last, fmt.Errorf("task %s latest worker evidence is not a matching PASS", id)
	}
	if _, e := git(root, "merge-base", "--is-ancestor", base, last.Result); e != nil {
		return last, fmt.Errorf("worker result does not descend from its base commit")
	}
	return last, nil
}

func exactKeys(value map[string]json.RawMessage, keys []string) bool {
	if len(value) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := value[key]; !ok {
			return false
		}
	}
	return true
}
func validUTC(value string) bool {
	parsed, e := time.Parse(time.RFC3339, value)
	return e == nil && parsed.Location() == time.UTC && strings.HasSuffix(value, "Z")
}

func cleanConfig(root string) error {
	tracked, e := git(root, "ls-files", "--error-unmatch", ".taskfactory/config.toml")
	if e != nil || tracked == "" {
		return fmt.Errorf(".taskfactory/config.toml must be tracked in HEAD")
	}
	if _, e = git(root, "diff", "--quiet", "HEAD", "--", ".taskfactory/config.toml"); e != nil {
		return fmt.Errorf(".taskfactory/config.toml differs from HEAD")
	}
	if _, e = git(root, "diff", "--cached", "--quiet", "HEAD", "--", ".taskfactory/config.toml"); e != nil {
		return fmt.Errorf(".taskfactory/config.toml is staged")
	}
	return nil
}
func cleanTree(root string) error {
	out, e := git(root, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return e
	}
	if out != "" {
		return fmt.Errorf("working tree has changes: %s", strings.TrimSpace(out))
	}
	return nil
}
func cleanMainRuntime(root, id, worktreeRoot string) error {
	out, e := gitOutputErr(root, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return e
	}
	statuses := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		if len(line) < 4 {
			return fmt.Errorf("unrelated main worktree change blocks integration: %s", line)
		}
		statuses[strings.TrimSpace(line[3:])] = line[:2]
	}
	worktreeRel, _ := filepath.Rel(root, worktreeRoot)
	for path := range statuses {
		if path == ".taskfactory/integration.lock" || path == ".taskfactory/claim.lock" || isEvidenceRuntimePath(path) || strings.HasPrefix(path, ".taskfactory/.claim-backup-") && strings.HasSuffix(path, ".tmp") {
			continue
		}
		if strings.HasPrefix(path, filepath.ToSlash(worktreeRel)+"/") {
			remainder := strings.TrimPrefix(path, filepath.ToSlash(worktreeRel)+"/")
			taskID := strings.SplitN(remainder, "/", 2)[0]
			if !idPattern.MatchString(taskID) || !retainedClaim(root, taskID) || !registeredClaimWorktree(root, filepath.Join(worktreeRoot, taskID)) {
				return fmt.Errorf("unregistered runtime worktree change blocks integration: %s", path)
			}
			continue
		}
		if strings.HasPrefix(path, "tasks/active/.claim-") && strings.HasSuffix(path, ".tmp") {
			continue
		}
		if strings.HasPrefix(path, "tasks/ready/") && statuses[path] == " D" {
			name := filepath.Base(path)
			activePath := filepath.Join(root, "tasks/active", name)
			if statuses[filepath.ToSlash(filepath.Join("tasks/active", name))] != "??" || !validClaimFile(root, activePath) {
				return fmt.Errorf("unrelated main worktree change blocks integration: %s", path)
			}
			continue
		}
		if strings.HasPrefix(path, "tasks/active/") && statuses[path] == "??" {
			name := filepath.Base(path)
			readyPath := filepath.ToSlash(filepath.Join("tasks/ready", name))
			if statuses[readyPath] != " D" || !validClaimFile(root, filepath.Join(root, path)) {
				return fmt.Errorf("unrelated main worktree change blocks integration: %s", path)
			}
			continue
		}
		return fmt.Errorf("unrelated main worktree change blocks integration: %s", path)
	}
	return nil
}

func validClaimFile(root, path string) bool {
	if ds := taskvalidate.Validate(root, filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))); len(ds) > 0 {
		return false
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return false
	}
	branch, worktree := "", ""
	inClaim := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "## Claim" {
			inClaim = true
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inClaim = false
		}
		if !inClaim {
			continue
		}
		if strings.HasPrefix(line, "Branch:") {
			branch = strings.TrimSpace(strings.TrimPrefix(line, "Branch:"))
		}
		if strings.HasPrefix(line, "Worktree:") {
			worktree = strings.TrimSpace(strings.TrimPrefix(line, "Worktree:"))
		}
	}
	return branch != "" && worktree != "" && registeredClaimWorktree(root, worktree)
}
func registeredClaimWorktree(root, path string) bool {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	listed, e := git(root, "worktree", "list", "--porcelain")
	if e != nil {
		return false
	}
	want, e := filepath.EvalSymlinks(path)
	if e != nil {
		return false
	}
	for _, block := range strings.Split(listed, "\n\n") {
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "worktree ") {
				got, e := filepath.EvalSymlinks(strings.TrimPrefix(line, "worktree "))
				if e == nil && filepath.Clean(got) == filepath.Clean(want) {
					return true
				}
			}
		}
	}
	return false
}
func checkRuntimeInventory(root, configuredWorktreeRoot string) error {
	dir := filepath.Join(root, ".taskfactory")
	configuredWorktreeRoot = filepath.Clean(configuredWorktreeRoot)
	entries, e := os.ReadDir(dir)
	if e != nil {
		return fmt.Errorf("inspect .taskfactory runtime paths: %w", e)
	}
	for _, entry := range entries {
		name := entry.Name()
		switch name {
		case "config.toml", "claim.lock", "integration.lock", "integration-stop.json", "integration-stop.json.tmp":
			if entry.IsDir() {
				return fmt.Errorf("unexpected runtime directory .taskfactory/%s", name)
			}
		case "evidence", "integration-evidence":
			if !entry.IsDir() {
				return fmt.Errorf("unexpected runtime path .taskfactory/%s", name)
			}
			children, err := os.ReadDir(filepath.Join(dir, name))
			if err != nil {
				return err
			}
			for _, child := range children {
				valid := false
				if name == "evidence" {
					valid = (strings.HasSuffix(child.Name(), ".jsonl") || strings.HasSuffix(child.Name(), ".lock")) && idPattern.MatchString(strings.TrimSuffix(strings.TrimSuffix(child.Name(), ".jsonl"), ".lock"))
				}
				if name == "integration-evidence" {
					valid = strings.HasSuffix(child.Name(), ".jsonl") && idPattern.MatchString(strings.TrimSuffix(child.Name(), ".jsonl"))
				}
				if !valid || child.IsDir() {
					return fmt.Errorf("unexpected runtime path .taskfactory/%s/%s", name, child.Name())
				}
			}
		case "worktrees":
			if !entry.IsDir() {
				return fmt.Errorf("unexpected runtime path .taskfactory/worktrees")
			}
			if configuredWorktreeRoot != filepath.Join(dir, "worktrees") {
				return fmt.Errorf("unexpected runtime path .taskfactory/worktrees (configured root is %s)", configuredWorktreeRoot)
			}
			children, err := os.ReadDir(filepath.Join(dir, name))
			if err != nil {
				return err
			}
			for _, child := range children {
				if !idPattern.MatchString(child.Name()) || !child.IsDir() {
					return fmt.Errorf("unexpected runtime worktree path .taskfactory/worktrees/%s", child.Name())
				}
				if !retainedClaim(root, child.Name()) || !registeredClaimWorktree(root, filepath.Join(dir, name, child.Name())) {
					return fmt.Errorf("unregistered runtime worktree .taskfactory/worktrees/%s", child.Name())
				}
			}
		default:
			if strings.HasPrefix(name, ".claim-backup-") && strings.HasSuffix(name, ".tmp") && !entry.IsDir() {
				continue
			}
			return fmt.Errorf("unexpected runtime path .taskfactory/%s", name)
		}
	}
	if configuredWorktreeRoot != filepath.Join(dir, "worktrees") {
		children, err := os.ReadDir(configuredWorktreeRoot)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect configured worktree root %s: %w", configuredWorktreeRoot, err)
		}
		for _, child := range children {
			if !idPattern.MatchString(child.Name()) || !child.IsDir() || !retainedClaim(root, child.Name()) || !registeredClaimWorktree(root, filepath.Join(configuredWorktreeRoot, child.Name())) {
				return fmt.Errorf("unexpected configured worktree path %s", filepath.Join(configuredWorktreeRoot, child.Name()))
			}
		}
	}
	return nil
}
func isEvidenceRuntimePath(path string) bool {
	for _, name := range []string{".taskfactory/evidence/", ".taskfactory/integration-evidence/"} {
		if strings.HasPrefix(path, name) {
			base := strings.TrimPrefix(path, name)
			suffix := ".jsonl"
			if strings.HasPrefix(name, ".taskfactory/evidence/") && strings.HasSuffix(base, ".lock") {
				suffix = ".lock"
			}
			return strings.HasSuffix(base, suffix) && idPattern.MatchString(strings.TrimSuffix(base, suffix))
		}
	}
	return false
}
func retainedClaim(root, id string) bool {
	for _, state := range []string{"active", "failed", "archive"} {
		entries, e := os.ReadDir(filepath.Join(root, "tasks", state))
		if e != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), id+"-") && strings.HasSuffix(entry.Name(), ".md") {
				b, e := os.ReadFile(filepath.Join(root, "tasks", state, entry.Name()))
				if e == nil && strings.Contains(string(b), "## Claim") {
					return true
				}
			}
		}
	}
	return false
}
func runCommand(dir, command string) (string, *int, error) {
	sh, e := exec.LookPath("sh")
	if e != nil {
		return "", nil, e
	}
	c := exec.Command(sh, "-c", command)
	c.Dir = dir
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	e = c.Run()
	if e == nil {
		v := 0
		return b.String(), &v, nil
	}
	var x *exec.ExitError
	if errors.As(e, &x) {
		v := x.ExitCode()
		return b.String(), &v, nil
	}
	return b.String(), nil, e
}
func git(dir string, args ...string) (string, error) {
	out, e := gitOutputErr(dir, args...)
	if e != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), e, strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}
func gitOutputErr(dir string, args ...string) (string, error) {
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, e := c.CombinedOutput()
	return string(out), e
}
func nextAttempt(root, id string) int {
	b, e := os.ReadFile(filepath.Join(root, ".taskfactory/integration-evidence", id+".jsonl"))
	if e != nil {
		return 1
	}
	return bytes.Count(b, []byte{'\n'}) + 1
}
func appendRecord(root, id string, r attempt) error {
	p := filepath.Join(root, ".taskfactory/integration-evidence", id+".jsonl")
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		return e
	}
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if _, e = f.Write(append(b, '\n')); e != nil {
		return e
	}
	return f.Sync()
}
func describeStop(root string) error {
	path := filepath.Join(root, ".taskfactory/integration-stop.json")
	b, e := os.ReadFile(path)
	if e != nil {
		return fmt.Errorf("integration stopped: inspect %s and repair main: %w", path, e)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil || !exactKeys(fields, []string{"main_commit", "command", "exit_code", "output", "error"}) {
		return fmt.Errorf("integration stopped: malformed %s; inspect it manually", path)
	}
	var rec struct {
		Main    string `json:"main_commit"`
		Command string `json:"command"`
		Exit    *int   `json:"exit_code"`
		Output  string `json:"output"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(b, &rec) != nil || !oidPattern.MatchString(rec.Main) || rec.Command == "" {
		return fmt.Errorf("integration stopped: malformed %s; inspect it manually", path)
	}
	return fmt.Errorf("integration stopped: main %s failed command %q; repair main and inspect %s", rec.Main, rec.Command, path)
}
func writeStop(root, main, command string, code *int, out string, startErr error) error {
	p := filepath.Join(root, ".taskfactory/integration-stop.json")
	if _, e := os.Lstat(p); e == nil {
		return fmt.Errorf("stop record already exists")
	}
	record := struct {
		Main    string `json:"main_commit"`
		Command string `json:"command"`
		Exit    *int   `json:"exit_code"`
		Output  string `json:"output"`
		Error   string `json:"error"`
	}{main, command, code, out, ""}
	if startErr != nil {
		record.Error = startErr.Error()
	}
	b, e := json.MarshalIndent(record, "", "  ")
	if e != nil {
		return e
	}
	tmp := p + ".tmp"
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	if _, e = f.Write(append(b, '\n')); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(tmp, p)
}

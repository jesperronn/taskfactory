// Package verify runs worker checks and appends immutable attempt evidence.
package verify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"taskfactory/internal/config"
	"taskfactory/internal/taskvalidate"
	"taskfactory/internal/ui"
)

type check struct {
	Source    string `json:"source"`
	Criterion string `json:"criterion"`
	Command   string `json:"command"`
	ExitCode  *int   `json:"exit_code"`
	Output    string `json:"output"`
	Error     string `json:"error"`
}
type record struct {
	TaskID       string   `json:"task_id"`
	Attempt      int      `json:"attempt"`
	RecordedAt   string   `json:"recorded_at"`
	Outcome      string   `json:"outcome"`
	Branch       string   `json:"branch"`
	BaseCommit   string   `json:"base_commit"`
	ResultCommit string   `json:"result_commit"`
	ChangedFiles []string `json:"changed_files"`
	Checks       []check  `json:"checks"`
	Note         string   `json:"note"`
}

var taskID = regexp.MustCompile(`^TF-[0-9]{3}$`)
var objectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// Result describes one recorded verification attempt. Outcome is empty when no
// evidence was appended, so callers print a summary only for recorded attempts.
type Result struct {
	ID      string
	Outcome string
	Total   int
	Failed  int
}

// Summary returns the one-line report for a recorded attempt, such as
// "verify TF-001: PASS (3 checks)". Only the outcome word is styled with p.
func (r Result) Summary(p ui.Painter) string {
	checks := "checks"
	if r.Total == 1 {
		checks = "check"
	}
	switch r.Outcome {
	case "PASS":
		return fmt.Sprintf("verify %s: %s (%d %s)", r.ID, p.Green("PASS"), r.Total, checks)
	case "FAILED":
		return fmt.Sprintf("verify %s: %s (%d of %d %s failed)", r.ID, p.Red("FAIL"), r.Failed, r.Total, checks)
	case "BLOCKED":
		return fmt.Sprintf("verify %s: %s (%d of %d %s could not start)", r.ID, p.Red("BLOCKED"), r.Failed, r.Total, checks)
	}
	return ""
}

// Run verifies an active claimed task in its worktree and appends one attempt.
func Run(root, id string) error {
	_, err := Verify(root, id)
	return err
}

// Verify is Run with a result describing the recorded attempt. The result is
// returned with a non-nil error when the attempt is recorded but not PASS.
func Verify(root, id string) (result Result, runErr error) {
	if !taskID.MatchString(id) {
		return Result{}, fmt.Errorf("%q is not a valid task ID (expected TF-NNN)", id)
	}
	cfg, err := config.Load(root)
	if err != nil {
		return Result{}, err
	}
	taskPath, err := activeTask(root, id)
	if err != nil {
		return Result{}, err
	}
	rel, _ := filepath.Rel(root, taskPath)
	if ds := taskvalidate.Validate(root, filepath.ToSlash(rel)); len(ds) > 0 {
		return Result{}, fmt.Errorf("active task %s is malformed: %s", id, ds[0])
	}
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return Result{}, err
	}
	sections := parseSections(string(data))
	claimValues := metadata(sections["Claim"])
	branch, base, worktree := claimValues["Branch"], claimValues["Base commit"], claimValues["Worktree"]
	if branch == "" || base == "" || worktree == "" {
		return Result{}, fmt.Errorf("active task %s has incomplete Claim metadata", id)
	}
	if _, err := os.Stat(worktree); err != nil {
		return Result{}, fmt.Errorf("task %s worktree %s is unavailable: %w", id, worktree, err)
	}
	top, err := git(worktree, "rev-parse", "--show-toplevel")
	canonicalTop, topErr := filepath.EvalSymlinks(top)
	canonicalWorktree, worktreeErr := filepath.EvalSymlinks(worktree)
	if err != nil || topErr != nil || worktreeErr != nil || filepath.Clean(canonicalTop) != filepath.Clean(canonicalWorktree) {
		return Result{}, fmt.Errorf("task %s claimed worktree is not a Git worktree at %s", id, worktree)
	}
	if err := ensureClaimedWorktree(root, canonicalWorktree, branch); err != nil {
		return Result{}, fmt.Errorf("task %s: %w", id, err)
	}
	actualBranch, err := git(worktree, "branch", "--show-current")
	if err != nil || actualBranch != branch {
		return Result{}, fmt.Errorf("task %s worktree branch %q does not match claimed branch %q", id, actualBranch, branch)
	}
	if _, err := git(worktree, "cat-file", "-e", base+"^{commit}"); err != nil {
		return Result{}, fmt.Errorf("task %s base commit %s is unavailable in worktree", id, base)
	}
	resultCommit, resultErr := git(worktree, "rev-parse", "HEAD")
	if resultErr != nil || resultCommit == base {
		resultCommit = ""
	}
	commands := []check{}
	for _, criterion := range parseCriteria(sections["Success criteria"]) {
		commands = append(commands, check{Source: "task", Criterion: criterion.id, Command: criterion.command})
	}
	for _, command := range cfg.Verification.Worker {
		commands = append(commands, check{Source: "worker", Command: command})
	}
	path := filepath.Join(root, ".taskfactory", "evidence", id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, fmt.Errorf("create evidence directory: %w", err)
	}
	lockPath := filepath.Join(root, ".taskfactory", "evidence", id+".lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return Result{}, fmt.Errorf("open evidence lock: %w", err)
	}
	defer lock.Close()
	if err := lockFile(lock); err != nil {
		return Result{}, fmt.Errorf("acquire evidence lock: %w", err)
	}
	defer func() {
		if err := unlockFile(lock); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("release evidence lock: %w", err))
		}
	}()
	prior, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("read evidence: %w", err)
	}
	attempt, err := validateEvidence(prior, id)
	if err != nil {
		return Result{}, err
	}
	entry := record{TaskID: id, Attempt: attempt + 1, RecordedAt: time.Now().UTC().Format(time.RFC3339), Outcome: "PASS", Branch: branch, BaseCommit: base, ResultCommit: resultCommit, Checks: []check{}, Note: ""}
	for _, c := range commands {
		out, code, startErr := runShell(worktree, c.Command)
		c.Output, c.ExitCode = out, code
		if startErr != nil {
			c.Error = startErr.Error()
			entry.Outcome = "BLOCKED"
		}
		entry.Checks = append(entry.Checks, c)
		if startErr != nil || (code != nil && *code != 0) {
			if entry.Outcome != "BLOCKED" {
				entry.Outcome = "FAILED"
			}
			break
		}
	}
	changed, err := changedFiles(worktree, base)
	if err != nil {
		return Result{}, err
	}
	entry.ChangedFiles = changed
	encoded, err := json.Marshal(entry)
	if err != nil {
		return Result{}, err
	}
	if err := appendSync(path, encoded); err != nil {
		return Result{}, fmt.Errorf("append verification evidence: %w", err)
	}
	result = Result{ID: id, Outcome: entry.Outcome, Total: len(commands)}
	if entry.Outcome != "PASS" {
		result.Failed = 1
		return result, fmt.Errorf("verification %s for task %s attempt %d; evidence recorded", entry.Outcome, id, entry.Attempt)
	}
	return result, nil
}

func activeTask(root, id string) (string, error) {
	dir := filepath.Join(root, "tasks", "active")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read active tasks: %w", err)
	}
	var found string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), id+"-") && strings.HasSuffix(e.Name(), ".md") {
			if found != "" {
				return "", fmt.Errorf("multiple active task files match %s", id)
			}
			found = filepath.Join(dir, e.Name())
		}
	}
	if found == "" {
		return "", fmt.Errorf("task %s is not active", id)
	}
	return found, nil
}
func parseSections(s string) map[string][]string {
	out := map[string][]string{}
	current := ""
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "## ") {
			current = strings.TrimPrefix(line, "## ")
			continue
		}
		if current != "" {
			out[current] = append(out[current], line)
		}
	}
	return out
}
func metadata(lines []string) map[string]string {
	m := map[string]string{}
	for _, line := range lines {
		for _, key := range []string{"Branch", "Base commit", "Worktree"} {
			prefix := key + ":"
			if strings.HasPrefix(line, prefix) {
				m[key] = strings.TrimSpace(strings.TrimPrefix(line, prefix))
			}
		}
	}
	return m
}
func parseCriteria(lines []string) []struct{ id, command string } {
	out := []struct{ id, command string }{}
	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			fields := strings.Fields(strings.TrimPrefix(line, "### "))
			if len(fields) > 0 {
				out = append(out, struct{ id, command string }{id: strings.TrimSuffix(fields[0], ":")})
			}
		}
		if strings.HasPrefix(line, "Check: ") && len(out) > 0 {
			out[len(out)-1].command = strings.TrimPrefix(line, "Check: ")
		}
	}
	return out
}
func git(dir string, args ...string) (string, error) {
	b, e := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), e, strings.TrimSpace(string(b)))
	}
	return strings.TrimSpace(string(b)), nil
}

func ensureClaimedWorktree(root, path, branch string) error {
	out, err := git(root, "worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("list repository worktrees: %w", err)
	}
	for _, block := range strings.Split(out, "\n\n") {
		var foundPath, foundBranch string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "worktree ") {
				foundPath = strings.TrimPrefix(line, "worktree ")
			}
			if strings.HasPrefix(line, "branch ") {
				foundBranch = strings.TrimPrefix(line, "branch refs/heads/")
			}
		}
		resolved, resolveErr := filepath.EvalSymlinks(foundPath)
		if resolveErr == nil && filepath.Clean(resolved) == filepath.Clean(path) && foundBranch == branch {
			return nil
		}
	}
	return fmt.Errorf("claimed path %s and branch %s are not a registered worktree", path, branch)
}
func changedFiles(dir, base string) ([]string, error) {
	a, e := exec.Command("git", "-C", dir, "diff", "--name-only", "-z", base, "--").Output()
	if e != nil {
		return nil, fmt.Errorf("list changed files: %w", e)
	}
	b, e := exec.Command("git", "-C", dir, "ls-files", "--others", "--exclude-standard", "-z").Output()
	if e != nil {
		return nil, fmt.Errorf("list untracked files: %w", e)
	}
	set := map[string]bool{}
	for _, raw := range [][]byte{a, b} {
		for _, v := range bytes.Split(raw, []byte{0}) {
			if len(v) > 0 {
				set[filepath.ToSlash(string(v))] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out, nil
}
func runShell(dir, command string) (string, *int, error) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		return "", nil, fmt.Errorf("start sh: %w", err)
	}
	c := exec.Command(shell, "-c", command)
	c.Dir = dir
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err = c.Run()
	if err == nil {
		v := 0
		return out.String(), &v, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		v := ee.ExitCode()
		return out.String(), &v, nil
	}
	return out.String(), nil, fmt.Errorf("start sh: %w", err)
}
func appendSync(path string, data []byte) error {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if e != nil {
		return e
	}
	defer f.Close()
	if _, e = f.Write(append(data, '\n')); e != nil {
		return e
	}
	return f.Sync()
}

func validateEvidence(data []byte, id string) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if data[len(data)-1] != '\n' {
		return 0, fmt.Errorf("evidence for %s is invalid: file does not end in LF", id)
	}
	lines := bytes.Split(data[:len(data)-1], []byte{'\n'})
	for i, line := range lines {
		if len(line) == 0 {
			return 0, fmt.Errorf("evidence for %s line %d is empty", id, i+1)
		}
		if err := validateRecord(line, id, i+1, i+1); err != nil {
			return 0, err
		}
	}
	return len(lines), nil
}
func validateRecord(line []byte, id string, number, want int) error {
	if err := rejectDuplicateJSONKeys(line); err != nil {
		return fmt.Errorf("evidence for %s line %d is malformed: %w", id, number, err)
	}
	var v map[string]json.RawMessage
	if err := json.Unmarshal(line, &v); err != nil {
		return fmt.Errorf("evidence for %s line %d is malformed: %w", id, number, err)
	}
	keys := []string{"task_id", "attempt", "recorded_at", "outcome", "branch", "base_commit", "result_commit", "changed_files", "checks", "note"}
	if len(v) != len(keys) {
		return fmt.Errorf("evidence for %s line %d has missing or unknown keys", id, number)
	}
	for _, key := range keys {
		if _, ok := v[key]; !ok {
			return fmt.Errorf("evidence for %s line %d is missing key %s", id, number, key)
		}
	}
	for _, k := range keys {
		if _, ok := v[k]; !ok {
			return fmt.Errorf("evidence for %s line %d is missing key %s", id, number, k)
		}
	}
	var r record
	if err := json.Unmarshal(line, &r); err != nil {
		return fmt.Errorf("evidence for %s line %d has wrong field types: %w", id, number, err)
	}
	if r.TaskID != id || r.Attempt != want || r.Attempt < 1 || !objectID.MatchString(r.BaseCommit) || (r.ResultCommit != "" && !objectID.MatchString(r.ResultCommit)) || strings.TrimSpace(r.Branch) == "" || !validUTC(r.RecordedAt) || (r.Outcome != "PASS" && r.Outcome != "FAILED" && r.Outcome != "BLOCKED") {
		return fmt.Errorf("evidence for %s line %d is logically inconsistent", id, number)
	}
	if len(r.Checks) == 0 {
		return fmt.Errorf("evidence for %s line %d has no command results", id, number)
	}
	for _, key := range []string{"task_id", "recorded_at", "outcome", "branch", "base_commit", "result_commit", "note"} {
		if !isJSONType(v[key], '"') {
			return fmt.Errorf("evidence for %s line %d field %s has wrong type", id, number, key)
		}
	}
	if !isJSONType(v["attempt"], '0') || !isJSONType(v["changed_files"], '[') || !isJSONType(v["checks"], '[') {
		return fmt.Errorf("evidence for %s line %d has wrong field type", id, number)
	}
	var rawFiles []json.RawMessage
	if err := json.Unmarshal(v["changed_files"], &rawFiles); err != nil {
		return fmt.Errorf("evidence for %s line %d changed_files has wrong type", id, number)
	}
	for i, raw := range rawFiles {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil || name == "" {
			return fmt.Errorf("evidence for %s line %d changed_files[%d] has wrong type or value", id, number, i)
		}
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
		if filepath.IsAbs(name) || clean != name || name == ".." || strings.HasPrefix(name, "../") {
			return fmt.Errorf("evidence for %s line %d changed_files contains non-relative path %q", id, number, name)
		}
		if i > 0 {
			var prev string
			_ = json.Unmarshal(rawFiles[i-1], &prev)
			if name <= prev {
				return fmt.Errorf("evidence for %s line %d changed_files must be sorted and unique", id, number)
			}
		}
	}
	for i, c := range r.Checks {
		if (c.Source != "task" && c.Source != "worker") || c.Command == "" || ((c.ExitCode == nil) != (c.Error != "")) {
			return fmt.Errorf("evidence for %s line %d has invalid check result", id, number)
		}
		if (c.Source == "worker" && c.Criterion != "") || (c.Source == "task" && c.Criterion == "") {
			return fmt.Errorf("evidence for %s line %d has invalid criterion source", id, number)
		}
		if i < len(r.Checks)-1 && (c.ExitCode == nil || *c.ExitCode != 0) {
			return fmt.Errorf("evidence for %s line %d continues after a failed check", id, number)
		}
	}
	switch r.Outcome {
	case "PASS":
		for _, c := range r.Checks {
			if c.ExitCode == nil || *c.ExitCode != 0 {
				return fmt.Errorf("evidence for %s line %d PASS outcome has a failed check", id, number)
			}
		}
	case "FAILED":
		if len(r.Checks) == 0 || r.Checks[len(r.Checks)-1].ExitCode == nil || *r.Checks[len(r.Checks)-1].ExitCode == 0 {
			return fmt.Errorf("evidence for %s line %d FAILED outcome has no failing command", id, number)
		}
	case "BLOCKED":
		if len(r.Checks) == 0 || r.Checks[len(r.Checks)-1].ExitCode != nil || r.Checks[len(r.Checks)-1].Error == "" {
			return fmt.Errorf("evidence for %s line %d BLOCKED outcome has no start error", id, number)
		}
	}
	var rawChecks []map[string]json.RawMessage
	_ = json.Unmarshal(v["checks"], &rawChecks)
	for i, raw := range rawChecks {
		if len(raw) != 6 {
			return fmt.Errorf("evidence for %s line %d check %d has missing or unknown keys", id, number, i+1)
		}
		for _, k := range []string{"source", "criterion", "command", "output", "error"} {
			if !isJSONType(raw[k], '"') {
				return fmt.Errorf("evidence for %s line %d check %d field %s has wrong type", id, number, i+1, k)
			}
		}
		if string(bytes.TrimSpace(raw["exit_code"])) != "null" && !isJSONType(raw["exit_code"], '0') {
			return fmt.Errorf("evidence for %s line %d check %d exit_code has wrong type", id, number, i+1)
		}
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if seen[key] {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = true
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
}

func isJSONType(raw json.RawMessage, prefix byte) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return false
	}
	if prefix == '0' {
		_, err := strconv.Atoi(string(raw))
		return err == nil
	}
	return raw[0] == prefix
}
func validUTC(s string) bool {
	t, e := time.Parse(time.RFC3339, s)
	return e == nil && t.Location() == time.UTC && strings.HasSuffix(s, "Z")
}

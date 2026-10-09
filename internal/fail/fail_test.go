package fail

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testBaseCommit = "0123456789abcdef0123456789abcdef01234567"

const testConfig = `protocol_version = 1

[workers]
max_parallel = 4

[git]
use_worktrees = true
worktree_root = ".taskfactory/worktrees"
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = ["go test ./..."]
integration = ["go test ./...", "go vet ./..."]
`

// taskContract returns a complete executable contract for id. When claimed is
// true it appends a Claim block that matches the fixture's configured worktree.
func taskContract(id, root string, claimed bool) string {
	body := "# " + id + ": Example task\n\n" +
		"## Goal\n\nMake the example change.\n\n" +
		"## Dependencies\n\nNone\n\n" +
		"## Scope\n\nTouch only the example file.\n\n" +
		"## Constraints\n\nUse only the standard library.\n\n" +
		"## Success criteria\n\n### C1: Example passes\n\nCheck: false\n\n" +
		"## Verification\n\nRun the check from the repository root.\n"
	if claimed {
		body += "\n## Claim\n\n" +
			"Owner: test-worker\n" +
			"Branch: task/" + id + "-impl\n" +
			"Worktree: " + filepath.Join(root, ".taskfactory", "worktrees", id) + "\n" +
			"Base commit: " + testBaseCommit + "\n" +
			"Started at: 2026-10-09T10:00:00Z\n"
	}
	return body
}

// newProject creates a temporary repository on main with the task tree and a
// committed active task. Global and system Git configuration are disabled, and
// the fixture repository sets commit.gpgsign to false in its own configuration.
func newProject(t *testing.T, claimed bool) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.name", "Test Worker")
	runGit(t, root, "config", "user.email", "worker@example.invalid")
	runGit(t, root, "config", "commit.gpgsign", "false")
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		dir := filepath.Join(root, "tasks", state)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, ".gitkeep"), "")
	}
	writeFile(t, filepath.Join(root, ".taskfactory", "config.toml"), testConfig)
	writeFile(t, filepath.Join(root, "tasks", "active", "TF-001-example.md"), taskContract("TF-001", root, claimed))
	runGit(t, root, "add", "--all")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	return root
}

// writeEvidence appends one valid attempt record for TF-001 with the given
// outcome. FAILED ends in a non-zero exit, BLOCKED in a start error.
func writeEvidence(t *testing.T, root, outcome string) []byte {
	t.Helper()
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
	entry := record{
		TaskID: "TF-001", Attempt: 1, RecordedAt: "2026-10-09T10:05:00Z",
		Outcome: outcome, Branch: "task/TF-001-impl", BaseCommit: testBaseCommit,
		ChangedFiles: []string{}, Checks: []check{{Source: "task", Criterion: "C1", Command: "false"}},
	}
	switch outcome {
	case "FAILED":
		code := 1
		entry.Checks[0].ExitCode = &code
	case "BLOCKED":
		entry.Checks[0].Command = "go test ./..."
		entry.Checks[0].Error = "start sh: no such file"
	default:
		t.Fatalf("unsupported outcome %q", outcome)
	}
	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	line = append(line, '\n')
	dir := filepath.Join(root, ".taskfactory", "evidence")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "TF-001.jsonl")
	if err := os.WriteFile(path, line, 0o600); err != nil {
		t.Fatal(err)
	}
	return line
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func commitCount(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, root, "rev-list", "--count", "HEAD"))
}

func TestMovesClaimedTaskWithFailedEvidence(t *testing.T) {
	root := newProject(t, true)
	writeEvidence(t, root, "FAILED")

	failed, err := Fail(root, "TF-001", Options{Outcome: "FAILED"})
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if failed != "tasks/failed/TF-001-example.md" {
		t.Fatalf("failed path = %q", failed)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "active", "TF-001-example.md")); !os.IsNotExist(err) {
		t.Fatalf("active file still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "failed", "TF-001-example.md")); err != nil {
		t.Fatalf("failed file missing: %v", err)
	}
	if got := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); got != "docs: mark TF-001 failed" {
		t.Fatalf("commit subject = %q", got)
	}
	names := runGit(t, root, "show", "--no-renames", "--name-status", "--format=", "HEAD")
	if !strings.Contains(names, "D\ttasks/active/TF-001-example.md") || !strings.Contains(names, "A\ttasks/failed/TF-001-example.md") {
		t.Fatalf("commit paths = %q", names)
	}
	if strings.Contains(names, ".taskfactory") {
		t.Fatalf("commit includes evidence: %q", names)
	}
}

func TestMovesBlockedTaskWithBlockedEvidence(t *testing.T) {
	root := newProject(t, true)
	writeEvidence(t, root, "BLOCKED")

	if _, err := Fail(root, "TF-001", Options{Outcome: "BLOCKED"}); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "failed", "TF-001-example.md")); err != nil {
		t.Fatalf("failed file missing: %v", err)
	}
}

func TestRefusesOutcomeMismatch(t *testing.T) {
	root := newProject(t, true)
	writeEvidence(t, root, "FAILED")
	before := commitCount(t, root)

	_, err := Fail(root, "TF-001", Options{Outcome: "BLOCKED"})
	if err == nil || !strings.Contains(err.Error(), "FAILED") || !strings.Contains(err.Error(), "BLOCKED") {
		t.Fatalf("err = %v, want refusal naming FAILED and BLOCKED", err)
	}
	if commitCount(t, root) != before {
		t.Fatal("refused command created a commit")
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "active", "TF-001-example.md")); err != nil {
		t.Fatalf("active file moved: %v", err)
	}
}

func TestRefusesReasonWhenEvidenceExists(t *testing.T) {
	root := newProject(t, true)
	writeEvidence(t, root, "FAILED")

	_, err := Fail(root, "TF-001", Options{Outcome: "FAILED", Reason: "stalled", ReasonSet: true})
	if err == nil || !strings.Contains(err.Error(), "--reason is refused") {
		t.Fatalf("err = %v, want --reason refusal", err)
	}
}

func TestRefusesMissingEvidenceWithoutReason(t *testing.T) {
	root := newProject(t, true)

	_, err := Fail(root, "TF-001", Options{Outcome: "BLOCKED"})
	if err == nil || !strings.Contains(err.Error(), "--reason") {
		t.Fatalf("err = %v, want missing evidence refusal", err)
	}
	writeFile(t, filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl"), "")
	if _, err := Fail(root, "TF-001", Options{Outcome: "BLOCKED"}); err == nil {
		t.Fatal("empty evidence file was accepted without --reason")
	}
}

func TestRefusesUnclaimedTask(t *testing.T) {
	root := newProject(t, false)
	writeEvidence(t, root, "FAILED")

	if _, err := Fail(root, "TF-001", Options{Outcome: "FAILED"}); err == nil {
		t.Fatal("unclaimed active task was accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "active", "TF-001-example.md")); err != nil {
		t.Fatalf("active file moved: %v", err)
	}
}

func TestRefusesTaskOutsideActive(t *testing.T) {
	root := newProject(t, true)
	runGit(t, root, "mv", "tasks/active/TF-001-example.md", "tasks/failed/TF-001-example.md")
	runGit(t, root, "commit", "-q", "-m", "move")
	writeEvidence(t, root, "FAILED")

	_, err := Fail(root, "TF-001", Options{Outcome: "FAILED"})
	if err == nil || !strings.Contains(err.Error(), "not tasks/active") {
		t.Fatalf("err = %v, want wrong-state refusal", err)
	}
}

func TestRefusesMissingTask(t *testing.T) {
	root := newProject(t, true)
	writeEvidence(t, root, "FAILED")

	_, err := Fail(root, "TF-002", Options{Outcome: "FAILED"})
	if err == nil || !strings.Contains(err.Error(), "no task file") {
		t.Fatalf("err = %v, want missing-file refusal", err)
	}
}

func TestRefusesStagedIndex(t *testing.T) {
	root := newProject(t, true)
	writeEvidence(t, root, "FAILED")
	writeFile(t, filepath.Join(root, "README.md"), "staged\n")
	runGit(t, root, "add", "README.md")

	_, err := Fail(root, "TF-001", Options{Outcome: "FAILED"})
	if err == nil || !strings.Contains(err.Error(), "staged paths") {
		t.Fatalf("err = %v, want staged-index refusal", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "active", "TF-001-example.md")); err != nil {
		t.Fatalf("active file moved: %v", err)
	}
}

func TestRefusesWhenValidationFailsAndRestoresFile(t *testing.T) {
	root := newProject(t, true)
	writeEvidence(t, root, "FAILED")
	active := filepath.Join(root, "tasks", "active", "TF-001-example.md")
	original := readFile(t, active)
	// An inbox file whose heading ID differs from its file name breaks the
	// whole-tree validation that runs after the move.
	writeFile(t, filepath.Join(root, "tasks", "inbox", "TF-999-bad.md"), "# TF-998: wrong\n")
	before := commitCount(t, root)

	_, err := Fail(root, "TF-001", Options{Outcome: "FAILED"})
	if err == nil || !strings.Contains(err.Error(), "invalid after failing") {
		t.Fatalf("err = %v, want validation failure", err)
	}
	if !bytes.Equal(readFile(t, active), original) {
		t.Fatal("active file bytes changed after restore")
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "failed", "TF-001-example.md")); !os.IsNotExist(err) {
		t.Fatalf("failed file left behind: %v", err)
	}
	if commitCount(t, root) != before {
		t.Fatal("validation failure created a commit")
	}
	if staged := strings.TrimSpace(runGit(t, root, "diff", "--cached", "--name-only")); staged != "" {
		t.Fatalf("index not clean after refusal: %q", staged)
	}
}

func TestRefusesWhenCommitFailsAndRestoresFile(t *testing.T) {
	root := newProject(t, true)
	writeEvidence(t, root, "FAILED")
	active := filepath.Join(root, "tasks", "active", "TF-001-example.md")
	original := readFile(t, active)
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	writeFile(t, hook, "#!/bin/sh\necho rejected by test hook >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	before := commitCount(t, root)

	_, err := Fail(root, "TF-001", Options{Outcome: "FAILED"})
	if err == nil || !strings.Contains(err.Error(), "commit failure") {
		t.Fatalf("err = %v, want commit failure", err)
	}
	if !bytes.Equal(readFile(t, active), original) {
		t.Fatal("active file bytes changed after restore")
	}
	if commitCount(t, root) != before {
		t.Fatal("failed commit moved HEAD")
	}
	if staged := strings.TrimSpace(runGit(t, root, "diff", "--cached", "--name-only")); staged != "" {
		t.Fatalf("index not reset after failed commit: %q", staged)
	}
}

func TestRetainsTaskAndEvidenceBytes(t *testing.T) {
	root := newProject(t, true)
	active := filepath.Join(root, "tasks", "active", "TF-001-example.md")
	original := readFile(t, active)
	evidence := writeEvidence(t, root, "FAILED")
	evidencePath := filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")

	if _, err := Fail(root, "TF-001", Options{Outcome: "FAILED"}); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	moved := readFile(t, filepath.Join(root, "tasks", "failed", "TF-001-example.md"))
	if !bytes.Equal(moved, original) {
		t.Fatal("moved task bytes differ from the active file")
	}
	if !strings.Contains(string(moved), "\n## Claim\n\nOwner: test-worker\n") {
		t.Fatal("Claim block is missing from the moved task")
	}
	if !bytes.Equal(readFile(t, evidencePath), evidence) {
		t.Fatal("evidence bytes changed")
	}
	if status := runGit(t, root, "status", "--porcelain", "--", ".taskfactory"); !strings.Contains(status, "?? .taskfactory/") {
		t.Fatalf("evidence is not untracked: %q", status)
	}
}

func TestReasonIsCommittedWithoutEvidence(t *testing.T) {
	root := newProject(t, true)
	reason := "worker stalled after 40 minutes"

	if _, err := Fail(root, "TF-001", Options{Outcome: "BLOCKED", Reason: reason, ReasonSet: true}); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	body := runGit(t, root, "log", "-1", "--format=%B")
	if !strings.Contains(body, "docs: mark TF-001 failed") || !strings.Contains(body, reason) {
		t.Fatalf("commit message = %q", body)
	}
	if _, err := os.Stat(filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("evidence file was created: %v", err)
	}
	if files := runGit(t, root, "show", "--no-renames", "--name-only", "--format=", "HEAD"); strings.Contains(files, "evidence") {
		t.Fatalf("commit touched evidence: %q", files)
	}
}

func TestReasonMustBeSingleLineText(t *testing.T) {
	for _, args := range [][]string{
		{"TF-001", "--outcome", "BLOCKED", "--reason", ""},
		{"TF-001", "--outcome", "BLOCKED", "--reason=   "},
		{"TF-001", "--outcome", "BLOCKED", "--reason", "one\ntwo"},
	} {
		_, _, err := ParseArgs(args)
		var usage UsageError
		if !errors.As(err, &usage) {
			t.Errorf("ParseArgs(%q) error = %v, want UsageError", args, err)
		}
	}
	if _, _, err := ParseArgs([]string{"TF-001", "--outcome", "FAILED", "--reason", "ok"}); err != nil {
		t.Fatalf("valid reason refused: %v", err)
	}
}

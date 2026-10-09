package requeue

import (
	"bytes"
	"encoding/json"
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

const taskFile = "TF-001-example.md"

func contract(id string) string {
	return "# " + id + ": Example task\n\n" +
		"## Goal\n\nMake the example change.\n\n" +
		"## Dependencies\n\nNone\n\n" +
		"## Scope\n\nTouch only the example file.\n\n" +
		"## Constraints\n\nUse only the standard library.\n\n" +
		"## Success criteria\n\n### C1: Example passes\n\nCheck: false\n\n" +
		"## Verification\n\nRun the check from the repository root.\n"
}

func claimBlock(id, root string) string {
	return "\n## Claim\n\n" +
		"Owner: test-worker\n" +
		"Branch: task/" + id + "-impl\n" +
		"Worktree: " + filepath.Join(root, ".taskfactory", "worktrees", id) + "\n" +
		"Base commit: " + testBaseCommit + "\n" +
		"Started at: 2026-10-09T10:00:00Z\n"
}

// newProject creates a temporary repository on main holding a committed failed
// task TF-001. Global and system Git configuration are disabled, and the
// fixture repository sets commit.gpgsign to false in its own configuration.
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
		writeFile(t, filepath.Join(root, "tasks", state, ".gitkeep"), "")
	}
	writeFile(t, filepath.Join(root, ".taskfactory", "config.toml"), testConfig)
	body := contract("TF-001")
	if claimed {
		body += claimBlock("TF-001", root)
	}
	writeFile(t, filepath.Join(root, "tasks", "failed", taskFile), body)
	runGit(t, root, "add", "--all")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	return root
}

// writeEvidence writes attempts consecutive FAILED records for TF-001 and
// returns the exact bytes.
func writeEvidence(t *testing.T, root string, attempts int) []byte {
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
	var all []byte
	for n := 1; n <= attempts; n++ {
		code := 1
		line, err := json.Marshal(record{
			TaskID: "TF-001", Attempt: n, RecordedAt: "2026-10-09T10:05:00Z",
			Outcome: "FAILED", Branch: "task/TF-001-impl", BaseCommit: testBaseCommit,
			ChangedFiles: []string{}, Checks: []check{{Source: "task", Criterion: "C1", Command: "false", ExitCode: &code}},
		})
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, append(line, '\n')...)
	}
	path := filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")
	writeFile(t, path, string(all))
	return all
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
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

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func commitCount(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, root, "rev-list", "--count", "HEAD"))
}

// assertTaskOnlyIn checks HEAD lists the task only under tasks/<state> and the
// task paths are clean in git status.
func assertTaskOnlyIn(t *testing.T, root, state string) {
	t.Helper()
	listing := runGit(t, root, "ls-tree", "-r", "HEAD", "--name-only")
	if !strings.Contains(listing, "tasks/"+state+"/"+taskFile+"\n") {
		t.Fatalf("HEAD lacks tasks/%s entry:\n%s", state, listing)
	}
	count := 0
	for _, line := range strings.Split(listing, "\n") {
		if strings.Contains(line, "TF-001") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("HEAD lists the task %d times:\n%s", count, listing)
	}
	if status := runGit(t, root, "status", "--short", "--", "tasks"); strings.TrimSpace(status) != "" {
		t.Fatalf("task paths dirty after requeue: %q", status)
	}
}

func assertUnchanged(t *testing.T, root string, original []byte, before string) {
	t.Helper()
	failed := filepath.Join(root, "tasks", "failed", taskFile)
	if !bytes.Equal(readFile(t, failed), original) {
		t.Fatal("failed task bytes changed")
	}
	for _, state := range []string{"ready", "inbox"} {
		if exists(filepath.Join(root, "tasks", state, taskFile)) {
			t.Fatalf("task left in tasks/%s", state)
		}
	}
	if commitCount(t, root) != before {
		t.Fatal("refused command created a commit")
	}
	if staged := strings.TrimSpace(runGit(t, root, "diff", "--cached", "--name-only")); staged != "" {
		t.Fatalf("index not clean: %q", staged)
	}
}

func TestRequeuesUnclaimedFailedTaskToReadyAndCommits(t *testing.T) {
	root := newProject(t, false)
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))

	res, err := Requeue(root, "TF-001", "ready")
	if err != nil {
		t.Fatalf("Requeue: %v", err)
	}
	if res.Path != "tasks/ready/"+taskFile || res.Aside != "" {
		t.Fatalf("result = %+v", res)
	}
	if !bytes.Equal(readFile(t, filepath.Join(root, "tasks", "ready", taskFile)), original) {
		t.Fatal("task bytes changed by the move")
	}
	if exists(filepath.Join(root, "tasks", "failed", taskFile)) {
		t.Fatal("failed file still exists")
	}
	if got := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); got != "docs: requeue TF-001 to ready" {
		t.Fatalf("commit subject = %q", got)
	}
	names := runGit(t, root, "show", "--no-renames", "--name-status", "--format=", "HEAD")
	if !strings.Contains(names, "D\ttasks/failed/"+taskFile) || !strings.Contains(names, "A\ttasks/ready/"+taskFile) || strings.Count(names, "\n") != 2 {
		t.Fatalf("commit paths = %q", names)
	}
	assertTaskOnlyIn(t, root, "ready")
}

func TestRequeuesStagesOnlyTaskPathsWithUnrelatedWorkingTreeChanges(t *testing.T) {
	root := newProject(t, false)
	writeFile(t, filepath.Join(root, "notes.txt"), "untracked\n")

	if _, err := Requeue(root, "TF-001", "ready"); err != nil {
		t.Fatalf("Requeue: %v", err)
	}
	names := runGit(t, root, "show", "--no-renames", "--name-status", "--format=", "HEAD")
	if strings.Contains(names, "notes.txt") {
		t.Fatalf("commit includes unrelated file: %q", names)
	}
	if status := runGit(t, root, "status", "--short"); strings.TrimSpace(status) != "?? notes.txt" {
		t.Fatalf("status = %q", status)
	}
	assertTaskOnlyIn(t, root, "ready")
}

func TestRequeuesUntrackedFailedFileStagesOnlyTarget(t *testing.T) {
	root := newProject(t, false)
	runGit(t, root, "rm", "-q", "--cached", "tasks/failed/"+taskFile)
	runGit(t, root, "commit", "-q", "-m", "untrack failed")

	if _, err := Requeue(root, "TF-001", "ready"); err != nil {
		t.Fatalf("Requeue: %v", err)
	}
	names := runGit(t, root, "show", "--no-renames", "--name-status", "--format=", "HEAD")
	if strings.TrimSpace(names) != "A\ttasks/ready/"+taskFile {
		t.Fatalf("commit paths = %q", names)
	}
	assertTaskOnlyIn(t, root, "ready")
}

func TestRequeuesRefusesStagedIndex(t *testing.T) {
	root := newProject(t, false)
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	writeFile(t, filepath.Join(root, "README.md"), "staged\n")
	runGit(t, root, "add", "README.md")
	before := commitCount(t, root)

	_, err := Requeue(root, "TF-001", "ready")
	if err == nil || !strings.Contains(err.Error(), "staged paths") {
		t.Fatalf("err = %v, want staged-index refusal", err)
	}
	if staged := strings.TrimSpace(runGit(t, root, "diff", "--cached", "--name-only")); staged != "README.md" {
		t.Fatalf("staged = %q, want only the unrelated path", staged)
	}
	if !bytes.Equal(readFile(t, filepath.Join(root, "tasks", "failed", taskFile)), original) || commitCount(t, root) != before {
		t.Fatal("refused command changed state")
	}
}

func TestRequeuesRefusesTaskOutsideFailedAndMissingTask(t *testing.T) {
	root := newProject(t, false)
	runGit(t, root, "mv", "tasks/failed/"+taskFile, "tasks/active/"+taskFile)
	runGit(t, root, "commit", "-q", "-m", "move to active")
	if _, err := Requeue(root, "TF-001", "ready"); err == nil || !strings.Contains(err.Error(), "not tasks/failed") {
		t.Fatalf("err = %v, want other-state refusal", err)
	}
	if _, err := Requeue(root, "TF-002", "ready"); err == nil || !strings.Contains(err.Error(), "no task file exists in tasks/failed") {
		t.Fatalf("err = %v, want missing-file refusal", err)
	}
}

func TestRequeuesRestoresEverythingWhenCommitFails(t *testing.T) {
	root := newProject(t, false)
	evidence := writeEvidence(t, root, 2)
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	writeFile(t, hook, "#!/bin/sh\necho rejected by test hook >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	before := commitCount(t, root)

	_, err := Requeue(root, "TF-001", "ready")
	if err == nil || !strings.Contains(err.Error(), "commit requeue") {
		t.Fatalf("err = %v, want commit failure", err)
	}
	assertUnchanged(t, root, original, before)
	if !bytes.Equal(readFile(t, filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")), evidence) {
		t.Fatal("evidence not restored under its original name")
	}
	if exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-2.jsonl")) {
		t.Fatal("aside file left behind")
	}
}

func TestRequeuesRestoresEverythingWhenTreeValidationFails(t *testing.T) {
	root := newProject(t, false)
	writeEvidence(t, root, 1)
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	// A heading ID that differs from its file name breaks whole-tree validation.
	writeFile(t, filepath.Join(root, "tasks", "inbox", "TF-999-bad.md"), "# TF-998: wrong\n")
	before := commitCount(t, root)

	_, err := Requeue(root, "TF-001", "ready")
	if err == nil || !strings.Contains(err.Error(), "invalid after requeueing") {
		t.Fatalf("err = %v, want validation failure", err)
	}
	assertUnchanged(t, root, original, before)
	if !exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")) || exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-1.jsonl")) {
		t.Fatal("evidence was renamed despite the refusal")
	}
}

func TestRefusesClaimedFailedTaskAndListsClaimedIDs(t *testing.T) {
	root := newProject(t, true)
	evidence := writeEvidence(t, root, 2)
	// A second claimed failed task and an unclaimed one: only claimed IDs are listed.
	writeFile(t, filepath.Join(root, "tasks", "failed", "TF-002-other.md"), contract("TF-002")+claimBlock("TF-002", root))
	writeFile(t, filepath.Join(root, "tasks", "failed", "TF-003-plain.md"), contract("TF-003"))
	runGit(t, root, "add", "--all")
	runGit(t, root, "commit", "-q", "-m", "more failed tasks")
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	before := commitCount(t, root)

	for _, target := range []string{"ready", "inbox"} {
		_, err := Requeue(root, "TF-001", target)
		if err == nil {
			t.Fatalf("--to %s: claimed task was requeued", target)
		}
		for _, want := range []string{"flagged for a human decision", "TF-001, TF-002"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("--to %s: error %q lacks %q", target, err, want)
			}
		}
		if strings.Contains(err.Error(), "TF-003") {
			t.Errorf("error lists unclaimed task: %v", err)
		}
		assertUnchanged(t, root, original, before)
	}
	if !bytes.Equal(readFile(t, filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")), evidence) {
		t.Fatal("evidence touched by a refusal")
	}
	ids, err := ClaimedFailed(root)
	if err != nil || strings.Join(ids, ",") != "TF-001,TF-002" {
		t.Fatalf("ClaimedFailed = %v, %v", ids, err)
	}
}

func TestRefusesUnsafeReadyContractAndLeavesTaskFailed(t *testing.T) {
	root := newProject(t, false)
	writeEvidence(t, root, 1)
	bad := strings.Replace(contract("TF-001"), "## Verification\n\nRun the check from the repository root.\n", "", 1)
	writeFile(t, filepath.Join(root, "tasks", "failed", taskFile), bad)
	runGit(t, root, "commit", "-q", "-am", "break contract")
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	before := commitCount(t, root)

	_, err := Requeue(root, "TF-001", "ready")
	if err == nil || !strings.Contains(err.Error(), "refusing to requeue") {
		t.Fatalf("err = %v, want contract refusal", err)
	}
	assertUnchanged(t, root, original, before)
	if !exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")) || exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-1.jsonl")) {
		t.Fatal("evidence was renamed despite the refusal")
	}
}

func TestRefusesUnsafeExistingAsidePath(t *testing.T) {
	root := newProject(t, false)
	evidence := writeEvidence(t, root, 2)
	aside := filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-2.jsonl")
	writeFile(t, aside, "older history\n")
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	before := commitCount(t, root)

	_, err := Requeue(root, "TF-001", "ready")
	if err == nil || !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "TF-001.attempts-2.jsonl") {
		t.Fatalf("err = %v, want aside collision refusal", err)
	}
	assertUnchanged(t, root, original, before)
	if string(readFile(t, aside)) != "older history\n" {
		t.Fatal("existing aside file was overwritten")
	}
	if !bytes.Equal(readFile(t, filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")), evidence) {
		t.Fatal("evidence changed by a refusal")
	}
}

func TestRefusesUnsafeTargetAlreadyExists(t *testing.T) {
	root := newProject(t, false)
	writeFile(t, filepath.Join(root, "tasks", "ready", taskFile), "occupied\n")
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))

	_, err := Requeue(root, "TF-001", "ready")
	if err == nil {
		t.Fatal("requeue over an existing ready file succeeded")
	}
	if !bytes.Equal(readFile(t, filepath.Join(root, "tasks", "failed", taskFile)), original) {
		t.Fatal("failed file changed")
	}
}

func TestResetsAttemptsByRenamingEvidenceAsideWithBytesKept(t *testing.T) {
	root := newProject(t, false)
	evidence := writeEvidence(t, root, 3)

	res, err := Requeue(root, "TF-001", "ready")
	if err != nil {
		t.Fatalf("Requeue: %v", err)
	}
	wantAside := ".taskfactory/evidence/TF-001.attempts-3.jsonl"
	if res.Aside != wantAside || res.Attempts != 3 {
		t.Fatalf("result = %+v, want aside %s", res, wantAside)
	}
	if exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")) {
		t.Fatal("original evidence file still present, so the counter was not reset")
	}
	if !bytes.Equal(readFile(t, filepath.Join(root, filepath.FromSlash(wantAside))), evidence) {
		t.Fatal("aside bytes differ from the original evidence")
	}
	body := runGit(t, root, "log", "-1", "--format=%b")
	if !strings.Contains(body, "counter reset") || !strings.Contains(body, wantAside) {
		t.Fatalf("commit body = %q", body)
	}
	if names := runGit(t, root, "show", "--no-renames", "--name-only", "--format=", "HEAD"); strings.Contains(names, ".taskfactory") {
		t.Fatalf("commit includes evidence: %q", names)
	}
	assertTaskOnlyIn(t, root, "ready")
}

func TestResetsAttemptsLeavesEmptyOrMissingEvidenceAlone(t *testing.T) {
	root := newProject(t, false)
	writeFile(t, filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl"), "")
	res, err := Requeue(root, "TF-001", "ready")
	if err != nil {
		t.Fatalf("Requeue: %v", err)
	}
	if res.Aside != "" {
		t.Fatalf("aside = %q for empty evidence", res.Aside)
	}
	if body := runGit(t, root, "log", "-1", "--format=%b"); strings.Contains(body, "reset") {
		t.Fatalf("commit body mentions a reset: %q", body)
	}
}

func TestResetsAttemptsRefusesInvalidEvidence(t *testing.T) {
	root := newProject(t, false)
	writeFile(t, filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl"), "not json\n")
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	before := commitCount(t, root)

	if _, err := Requeue(root, "TF-001", "ready"); err == nil {
		t.Fatal("invalid evidence was accepted")
	}
	assertUnchanged(t, root, original, before)
}

func TestInboxTargetMovesUnclaimedTaskAndSkipsContractCheck(t *testing.T) {
	root := newProject(t, false)
	// An incomplete contract is allowed in the inbox.
	loose := "# TF-001: Example task\n\n## Goal\n\nOnly a goal.\n"
	writeFile(t, filepath.Join(root, "tasks", "failed", taskFile), loose)
	runGit(t, root, "commit", "-q", "-am", "loosen")
	evidence := writeEvidence(t, root, 1)

	res, err := Requeue(root, "TF-001", "inbox")
	if err != nil {
		t.Fatalf("Requeue: %v", err)
	}
	if res.Path != "tasks/inbox/"+taskFile {
		t.Fatalf("path = %q", res.Path)
	}
	if got := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); got != "docs: requeue TF-001 to inbox" {
		t.Fatalf("commit subject = %q", got)
	}
	if string(readFile(t, filepath.Join(root, "tasks", "inbox", taskFile))) != loose {
		t.Fatal("task bytes changed")
	}
	if !bytes.Equal(readFile(t, filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-1.jsonl")), evidence) {
		t.Fatal("evidence not renamed aside")
	}
	assertTaskOnlyIn(t, root, "inbox")
}

func TestParseArgsUsageErrors(t *testing.T) {
	good := []struct {
		args   []string
		id, to string
	}{
		{[]string{"TF-001"}, "TF-001", "ready"},
		{[]string{"TF-001", "--to", "inbox"}, "TF-001", "inbox"},
		{[]string{"--to=ready", "TF-001"}, "TF-001", "ready"},
	}
	for _, tt := range good {
		id, to, err := ParseArgs(tt.args)
		if err != nil || id != tt.id || to != tt.to {
			t.Errorf("ParseArgs(%v) = %q, %q, %v", tt.args, id, to, err)
		}
	}
	bad := [][]string{
		{}, {"--to", "ready"}, {"TF-001", "--to"}, {"TF-001", "--to", "active"},
		{"TF-001", "TF-002"}, {"TF-001", "--force"}, {"TF-001", "--to", "ready", "--to", "inbox"}, {"nope"},
	}
	for _, args := range bad {
		if _, _, err := ParseArgs(args); err == nil {
			t.Errorf("ParseArgs(%v) succeeded", args)
		} else if _, ok := err.(UsageError); !ok {
			t.Errorf("ParseArgs(%v) error %T is not UsageError", args, err)
		}
	}
}

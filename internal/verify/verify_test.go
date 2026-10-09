package verify

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRunOrdersChecksStopsOnFailureAndRecordsOutput(t *testing.T) {
	root, worktree, base := fixture(t, []string{"echo task-one >> order; printf task-output", "echo task-two >> order; exit 7"}, []string{"echo worker >> order"})
	if err := os.WriteFile(filepath.Join(worktree, "changed.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "TF-099"); err == nil || !strings.Contains(err.Error(), "FAILED") {
		t.Fatalf("failed verification error = %v", err)
	}
	order, err := os.ReadFile(filepath.Join(worktree, "order"))
	if err != nil {
		t.Fatal(err)
	}
	if string(order) != "task-one\ntask-two\n" {
		t.Fatalf("order = %q", order)
	}
	data, err := os.ReadFile(filepath.Join(root, ".taskfactory", "evidence", "TF-099.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var got record
	if err := json.Unmarshal(bytesLine(data), &got); err != nil {
		t.Fatal(err)
	}
	if got.Outcome != "FAILED" || got.Attempt != 1 || len(got.Checks) != 2 || got.Checks[0].Source != "task" || got.Checks[0].Criterion != "C1" || got.Checks[0].Output != "task-output" || got.Checks[1].ExitCode == nil || *got.Checks[1].ExitCode != 7 {
		t.Fatalf("record = %#v", got)
	}
	if got.BaseCommit != base || len(got.ChangedFiles) != 2 || got.ChangedFiles[0] != "changed.txt" || got.ChangedFiles[1] != "order" {
		t.Fatalf("changed files = %#v", got.ChangedFiles)
	}
}

func TestRunExecutesWorkerCommandsAfterAllCriteria(t *testing.T) {
	root, worktree, _ := fixture(t, []string{"echo task >> sequence", "echo criterion >> sequence"}, []string{"echo worker >> sequence"})
	if err := Run(root, "TF-099"); err != nil {
		t.Fatal(err)
	}
	sequence, err := os.ReadFile(filepath.Join(worktree, "sequence"))
	if err != nil {
		t.Fatal(err)
	}
	if string(sequence) != "task\ncriterion\nworker\n" {
		t.Fatalf("sequence = %q", sequence)
	}
	data, err := os.ReadFile(filepath.Join(root, ".taskfactory", "evidence", "TF-099.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var got record
	if err := json.Unmarshal(bytesLine(data), &got); err != nil {
		t.Fatal(err)
	}
	if got.Outcome != "PASS" || len(got.Checks) != 3 || got.Checks[2].Source != "worker" {
		t.Fatalf("record = %#v", got)
	}
}

func TestRunResultCommitTracksCommittedWorktreeHead(t *testing.T) {
	t.Run("unchanged", func(t *testing.T) {
		root, _, base := fixture(t, []string{"true"}, []string{"true"})
		if err := Run(root, "TF-099"); err != nil {
			t.Fatal(err)
		}
		got := readRecord(t, root)
		if got.ResultCommit != "" || got.BaseCommit != base {
			t.Fatalf("result=%q base=%q", got.ResultCommit, got.BaseCommit)
		}
	})
	t.Run("dirty uncommitted", func(t *testing.T) {
		root, worktree, _ := fixture(t, []string{"true"}, []string{"true"})
		if err := os.WriteFile(filepath.Join(worktree, "dirty.txt"), []byte("uncommitted\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run(root, "TF-099"); err != nil {
			t.Fatal(err)
		}
		got := readRecord(t, root)
		if got.ResultCommit != "" || !contains(got.ChangedFiles, "dirty.txt") {
			t.Fatalf("result=%q changed=%v", got.ResultCommit, got.ChangedFiles)
		}
	})
	t.Run("committed", func(t *testing.T) {
		root, worktree, base := fixture(t, []string{"true"}, []string{"true"})
		if err := os.WriteFile(filepath.Join(worktree, "committed.txt"), []byte("committed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"add", "committed.txt"}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "-m", "worker result"}} {
			cmd := exec.Command("git", append([]string{"-C", worktree}, args...)...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		headCmd := exec.Command("git", "-C", worktree, "rev-parse", "HEAD")
		headBytes, err := headCmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		head := strings.TrimSpace(string(headBytes))
		if err := Run(root, "TF-099"); err != nil {
			t.Fatal(err)
		}
		got := readRecord(t, root)
		if got.ResultCommit != head || got.ResultCommit == base {
			t.Fatalf("result=%q want HEAD %q and base %q", got.ResultCommit, head, base)
		}
	})
}

func readRecord(t *testing.T, root string) record {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".taskfactory", "evidence", "TF-099.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var got record
	if err := json.Unmarshal(bytesLine(data), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestRunAppendsConcurrentAttemptsAndRejectsCorruption(t *testing.T) {
	root, _, _ := fixture(t, []string{"true"}, []string{"true"})
	if err := Run(root, "TF-099"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".taskfactory", "evidence", "TF-099.jsonl")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- Run(root, "TF-099") }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	all, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(all), string(first)) {
		t.Fatal("prior evidence bytes changed")
	}
	attempt, err := validateEvidence(all, "TF-099")
	if err != nil || attempt != 3 {
		t.Fatalf("attempts=%d err=%v", attempt, err)
	}
	bad := append(append([]byte(nil), all...), []byte("{bad json}\n")...)
	if err := os.WriteFile(path, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := Run(root, "TF-099"); err == nil {
		t.Fatal("expected corrupt evidence error")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("corrupt evidence was modified")
	}
}

func TestRunRejectsDuplicateEvidenceKeysWithoutChangingBytes(t *testing.T) {
	for name, duplicate := range map[string]string{
		"top-level":    strings.Replace(validRecordJSON(), `"task_id":"TF-099",`, `"task_id":"TF-099","task_id":"TF-099",`, 1),
		"nested-check": strings.Replace(validRecordJSON(), `"source":"worker",`, `"source":"worker","source":"worker",`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			root, worktree, _ := fixture(t, []string{"echo ran > ran"}, []string{"true"})
			path := filepath.Join(root, ".taskfactory", "evidence", "TF-099.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			before := []byte(duplicate + "\n")
			if err := os.WriteFile(path, before, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := Run(root, "TF-099"); err == nil || !strings.Contains(err.Error(), "duplicate object key") {
				t.Fatalf("duplicate-key error = %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatal("duplicate-key evidence bytes changed")
			}
			if _, err := os.Stat(filepath.Join(worktree, "ran")); !os.IsNotExist(err) {
				t.Fatalf("task command ran before evidence validation: %v", err)
			}
		})
	}
}

func TestValidateEvidenceRejectsWrongIdentityAndUnknownKeys(t *testing.T) {
	base := validRecordJSON()
	invalid := []string{
		"{bad json",
		strings.Replace(base, "TF-099", "TF-098", 1),
		strings.Replace(base, `"note":""}`, `"note":"","extra":true}`, 1),
		strings.Replace(base, `,"note":""`, ``, 1),
		strings.Replace(base, `"attempt":1`, `"attempt":"1"`, 1),
		strings.Replace(base, `"branch":"feature/TF-099"`, `"branch":3`, 1),
	}
	for _, line := range invalid {
		if _, err := validateEvidence([]byte(line+"\n"), "TF-099"); err == nil {
			t.Fatalf("accepted invalid evidence %s", line)
		}
	}
	second := strings.Replace(base, `"attempt":1`, `"attempt":3`, 1)
	if _, err := validateEvidence([]byte(base+"\n"+base+"\n"), "TF-099"); err == nil {
		t.Fatal("accepted duplicate attempt")
	}
	if _, err := validateEvidence([]byte(base+"\n"+second+"\n"), "TF-099"); err == nil {
		t.Fatal("accepted attempt gap")
	}
}

func validRecordJSON() string {
	return `{"task_id":"TF-099","attempt":1,"recorded_at":"2026-10-08T12:00:00Z","outcome":"PASS","branch":"feature/TF-099","base_commit":"0123456789abcdef0123456789abcdef01234567","result_commit":"","changed_files":[],"checks":[{"source":"worker","criterion":"","command":"true","exit_code":0,"output":"","error":""}],"note":""}`
}

func TestRunRepairCreatesNextAttempt(t *testing.T) {
	root, _, _ := fixture(t, []string{"false"}, []string{"true"})
	if err := Run(root, "TF-099"); err == nil || !strings.Contains(err.Error(), "FAILED") {
		t.Fatalf("first failed verification error = %v", err)
	}
	taskPath := filepath.Join(root, "tasks", "active", "TF-099-fixture.md")
	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(taskPath, []byte(strings.Replace(string(data), "Check: false", "Check: true", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "TF-099"); err != nil {
		t.Fatal(err)
	}
	evidence, err := os.ReadFile(filepath.Join(root, ".taskfactory", "evidence", "TF-099.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(evidence), "\n"), "\n")
	var first, second record
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatal(err)
	}
	if first.Outcome != "FAILED" || second.Outcome != "PASS" || second.Attempt != 2 {
		t.Fatalf("records = %#v, %#v", first, second)
	}
}

func TestRunRejectsMissingWorktreeAndMalformedClaim(t *testing.T) {
	root, worktree, _ := fixture(t, []string{"true"}, []string{"true"})
	if err := os.RemoveAll(worktree); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "TF-099"); err == nil || !strings.Contains(err.Error(), "worktree") {
		t.Fatalf("missing worktree error = %v", err)
	}
	root, _, _ = fixture(t, []string{"true"}, []string{"true"})
	taskPath := filepath.Join(root, "tasks", "active", "TF-099-fixture.md")
	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "Branch: feature/TF-099", "Branch: ", 1))
	if err := os.WriteFile(taskPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "TF-099"); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed claim error = %v", err)
	}
}

func TestRunRecordsBlockedShellStartError(t *testing.T) {
	root, _, _ := fixture(t, []string{"true"}, []string{"true"})
	oldPath := os.Getenv("PATH")
	bin := t.TempDir()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(gitPath, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("PATH", bin); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("PATH", oldPath)
	if err := Run(root, "TF-099"); err == nil || !strings.Contains(err.Error(), "BLOCKED") {
		t.Fatalf("blocked verification error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".taskfactory", "evidence", "TF-099.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var got record
	if err := json.Unmarshal(bytesLine(data), &got); err != nil {
		t.Fatal(err)
	}
	if got.Outcome != "BLOCKED" || len(got.Checks) != 1 || got.Checks[0].ExitCode != nil || !strings.Contains(got.Checks[0].Error, "start sh") {
		t.Fatalf("blocked record = %#v", got)
	}
}

func fixture(t *testing.T, criteria, workers []string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	run := func(dir string, args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	_ = run(root, "init", "-b", "main")
	_ = run(root, "config", "user.email", "test@example.com")
	_ = run(root, "config", "user.name", "Test")
	_ = run(root, "config", "commit.gpgsign", "false")
	if err := os.MkdirAll(filepath.Join(root, ".taskfactory"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `protocol_version = 1
[git]
worktree_root = ".taskfactory/worktrees"
[verification]
worker = [` + quoteList(workers) + `]
integration = ["true"]
`
	if err := os.WriteFile(filepath.Join(root, ".taskfactory", "config.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"inbox", "ready", "active", "failed", "archive"} {
		if err := os.MkdirAll(filepath.Join(root, "tasks", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", ".")
	run(root, "commit", "-m", "base")
	base := run(root, "rev-parse", "HEAD")
	branch := "feature/TF-099"
	run(root, "branch", branch, base)
	worktree := filepath.Join(root, ".taskfactory", "worktrees", "TF-099")
	run(root, "worktree", "add", worktree, branch)
	var crit strings.Builder
	for i, c := range criteria {
		crit.WriteString("### C" + string(rune('1'+i)) + ": check\n\nCheck: " + c + "\n\n")
	}
	task := `# TF-099: Verification fixture

## Goal

Exercise verification.

## Dependencies

None

## Scope

Test fixture.

## Constraints

No lifecycle changes.

## Success criteria

` + crit.String() + `## Verification

Fixture checks.

## Claim

Owner: test
Branch: ` + branch + `
Worktree: ` + worktree + `
Base commit: ` + base + `
Started at: 2026-10-08T12:00:00Z
`
	if err := os.WriteFile(filepath.Join(root, "tasks", "active", "TF-099-fixture.md"), []byte(task), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, worktree, base
}
func quoteList(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
	}
	return strings.Join(quoted, ", ")
}
func bytesLine(data []byte) []byte { return []byte(strings.SplitN(string(data), "\n", 2)[0]) }

package integrate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestIntegrateAdvancesMainAndArchivesOnlyRequestedTask(t *testing.T) {
	f := newFixture(t)
	if err := Run(f.root, "TF-901"); err != nil {
		status, _ := git(f.root, "status", "--short")
		evidence, _ := os.ReadFile(filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl"))
		t.Fatalf("%v; status=%q evidence=%s", err, status, evidence)
	}
	parent, _ := git(f.root, "rev-parse", "HEAD^")
	if parent != f.result {
		t.Fatalf("archive commit parent=%s want candidate %s", parent, f.result)
	}
	if _, err := os.Stat(filepath.Join(f.root, "tasks/archive/TF-901-example.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.root, "tasks/active/TF-901-example.md")); !os.IsNotExist(err) {
		t.Fatalf("active task remains: %v", err)
	}
	changed, err := git(f.root, "show", "--no-renames", "--pretty=format:", "--name-only", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(changed) != "tasks/archive/TF-901-example.md\ntasks/ready/TF-901-example.md" {
		t.Fatalf("archive commit paths: %q", changed)
	}
	data, err := os.ReadFile(filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl"))
	if err != nil || !strings.Contains(string(data), `"outcome":"PASS"`) {
		t.Fatalf("evidence=%s err=%v", data, err)
	}
}

func TestArchiveCommitDeletesTrackedActiveTask(t *testing.T) {
	f := newFixture(t)
	run(t, f.root, "git", "add", "-A", "--", "tasks/ready/TF-901-example.md", "tasks/active/TF-901-example.md")
	run(t, f.root, "git", "commit", "-m", "claim task")
	if err := Run(f.root, "TF-901"); err != nil {
		t.Fatal(err)
	}
	changed, err := git(f.root, "show", "--no-renames", "--pretty=format:", "--name-only", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(changed) != "tasks/active/TF-901-example.md\ntasks/archive/TF-901-example.md" {
		t.Fatalf("archive commit paths: %q", changed)
	}
}

func TestPostMergeMainFailureStopsFutureIntegration(t *testing.T) {
	f := newFixture(t)
	configPath := filepath.Join(f.root, ".taskfactory/config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.TrimSuffix(string(data), "\n") + "\nmain=[\"false\"]\n"
	if err = os.WriteFile(configPath, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, f.root, "git", "add", "--", ".taskfactory/config.toml")
	run(t, f.root, "git", "commit", "-m", "configure main check")
	if err = Run(f.root, "TF-901"); err == nil {
		t.Fatal("Run unexpectedly succeeded")
	}
	main, _ := git(f.root, "rev-parse", "refs/heads/main")
	if main == f.result {
		t.Fatal("expected main to include the rebase after config commit")
	}
	if _, err = os.Stat(filepath.Join(f.root, "tasks/active/TF-901-example.md")); err != nil {
		t.Fatalf("active task moved: %v", err)
	}
	stop, err := os.ReadFile(filepath.Join(f.root, ".taskfactory/integration-stop.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"\"command\": \"false\"", "\"exit_code\": 1", "\"main_commit\": \"" + main + "\""} {
		if !strings.Contains(string(stop), want) {
			t.Fatalf("stop record missing %s: %s", want, stop)
		}
	}
	if err = Run(f.root, "TF-901"); err == nil || !strings.Contains(err.Error(), "integration stopped") {
		t.Fatalf("second integrate err=%v", err)
	}
}

func TestMainMovementDuringChecksForcesRebaseAndRerun(t *testing.T) {
	f := newFixture(t)
	marker := filepath.Join(t.TempDir(), "moved")
	command := "echo check >> " + marker + "; if mkdir " + marker + ".once; then git -C " + f.root + " -c user.name=Test -c user.email=test@example.invalid commit --allow-empty -m external-move; fi"
	configureIntegrationCommand(t, f.root, command)
	if err := Run(f.root, "TF-901"); err != nil {
		t.Fatal(err)
	}
	checks, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(checks), "check") != 2 {
		t.Fatalf("integration checks ran %d times after main moved; output=%q", strings.Count(string(checks), "check"), checks)
	}
	parent, err := git(f.root, "rev-parse", "HEAD^")
	if err != nil {
		t.Fatal(err)
	}
	if parent == f.result {
		t.Fatal("candidate was not rebased onto moved main before archive")
	}
}

func TestIntegrationFailureLeavesMainAndTaskStateUnchanged(t *testing.T) {
	f := newFixture(t)
	configureIntegrationCommand(t, f.root, "false")
	main, _ := git(f.root, "rev-parse", "refs/heads/main")
	if err := Run(f.root, "TF-901"); err == nil {
		t.Fatal("Run unexpectedly succeeded")
	}
	got, _ := git(f.root, "rev-parse", "refs/heads/main")
	if got != main {
		t.Fatalf("main moved from %s to %s", main, got)
	}
	if _, err := os.Stat(filepath.Join(f.root, "tasks/active/TF-901-example.md")); err != nil {
		t.Fatalf("task left active: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl"))
	if err != nil || !strings.Contains(string(b), `"stage":"integration_check"`) {
		t.Fatalf("integration failure evidence=%s err=%v", b, err)
	}
}

func TestIntegrationLockSerializesCalls(t *testing.T) {
	f := newFixture(t)
	p := filepath.Join(f.root, ".taskfactory/integration.lock")
	lock, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- Run(f.root, "TF-901") }()
	select {
	case err = <-done:
		t.Fatalf("integration did not wait for lock: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	_ = lock.Close()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("integration stayed blocked after lock release")
	}
}

func TestArchiveCommitFailureRestoresActiveTask(t *testing.T) {
	f := newFixture(t)
	hook := filepath.Join(f.root, ".git/hooks/pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho rejected >&2\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := Run(f.root, "TF-901"); err == nil {
		t.Fatal("Run unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(f.root, "tasks/active/TF-901-example.md")); err != nil {
		t.Fatalf("active task not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.root, "tasks/archive/TF-901-example.md")); !os.IsNotExist(err) {
		t.Fatalf("archive task remains: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl"))
	if err != nil || !strings.Contains(string(b), `"stage":"archive"`) {
		t.Fatalf("archive evidence=%s err=%v", b, err)
	}
}

func TestIntegrationEvidenceAppendFailureDoesNotMoveMain(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	before, _ := git(f.root, "rev-parse", "refs/heads/main")
	err := Run(f.root, "TF-901")
	if err == nil || !strings.Contains(err.Error(), "append integration evidence") {
		t.Fatalf("append error=%v", err)
	}
	after, _ := git(f.root, "rev-parse", "refs/heads/main")
	if after != before {
		t.Fatalf("main moved: %s -> %s", before, after)
	}
}

func TestFinalPassEvidenceAppendFailurePreservesArchiveCommit(t *testing.T) {
	f := newFixture(t)
	configureMainCommand(t, f.root, "mkdir -p .taskfactory/integration-evidence/TF-901.jsonl")
	mainBefore, err := git(f.root, "rev-parse", "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	err = Run(f.root, "TF-901")
	if err == nil || !strings.Contains(err.Error(), "archive committed but append PASS integration evidence failed") {
		t.Fatalf("Run error = %v; want final evidence append failure", err)
	}
	if _, err := os.Stat(filepath.Join(f.root, "tasks/active/TF-901-example.md")); !os.IsNotExist(err) {
		t.Fatalf("active task should remain archived after evidence failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.root, "tasks/archive/TF-901-example.md")); err != nil {
		t.Fatalf("archive task missing after evidence failure: %v", err)
	}
	parent, err := git(f.root, "rev-parse", "HEAD^")
	if err != nil {
		t.Fatalf("read archive commit parent: %v", err)
	}
	if _, err = git(f.root, "merge-base", "--is-ancestor", mainBefore, parent); err != nil {
		t.Fatalf("rebased candidate %s is not based on current main %s", parent, mainBefore)
	}
	feature, err := git(f.root, "show", parent+":feature.txt")
	if err != nil || feature != "candidate" {
		t.Fatalf("archive parent candidate feature = %q, err = %v", feature, err)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl")); err != nil {
		t.Fatalf("failure setup directory missing: %v", err)
	}
}

func configureMainCommand(t *testing.T, root, command string) {
	t.Helper()
	p := filepath.Join(root, ".taskfactory/config.toml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.TrimSuffix(string(b), "\n") + "\nmain=[\"" + command + "\"]\n"
	if err = os.WriteFile(p, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", "--", ".taskfactory/config.toml")
	run(t, root, "git", "commit", "-m", "configure main check")
}

func TestMismatchedWorkerResultAndUnrelatedMainFileAreRejected(t *testing.T) {
	for _, mode := range []string{"mismatched worker result", "unrelated main file"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			before, _ := git(f.root, "rev-parse", "refs/heads/main")
			if mode == "mismatched worker result" {
				p := filepath.Join(f.root, ".taskfactory/evidence/TF-901.jsonl")
				b, e := os.ReadFile(p)
				if e != nil {
					t.Fatal(e)
				}
				updated := strings.Replace(string(b), f.result, strings.Repeat("0", 40), 1)
				if e = os.WriteFile(p, []byte(updated), 0600); e != nil {
					t.Fatal(e)
				}
			} else {
				write(t, f.root, "user-file.txt", "unrelated\n")
			}
			if err := Run(f.root, "TF-901"); err == nil {
				t.Fatal("Run unexpectedly succeeded")
			}
			after, _ := git(f.root, "rev-parse", "refs/heads/main")
			if after != before {
				t.Fatalf("main moved: %s -> %s", before, after)
			}
		})
	}
}

func TestStopTemporarySiblingBlocksIntegration(t *testing.T) {
	f := newFixture(t)
	p := filepath.Join(f.root, ".taskfactory/integration-stop.json.tmp")
	if err := os.WriteFile(p, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(f.root, "TF-901"); err == nil || !strings.Contains(err.Error(), "integration-stop.json.tmp") {
		t.Fatalf("Run error=%v", err)
	}
	main, _ := git(f.root, "rev-parse", "refs/heads/main")
	if main == f.result {
		t.Fatal("main unexpectedly advanced")
	}
}

func TestRebaseConflictDoesNotAdvanceMainOrArchive(t *testing.T) {
	f := newFixture(t)
	write(t, f.root, "feature.txt", "conflicting main change\n")
	run(t, f.root, "git", "add", "--", "feature.txt")
	run(t, f.root, "git", "commit", "-m", "conflict")
	before, _ := git(f.root, "rev-parse", "refs/heads/main")
	if err := Run(f.root, "TF-901"); err == nil {
		t.Fatal("Run unexpectedly succeeded")
	}
	after, _ := git(f.root, "rev-parse", "refs/heads/main")
	if after != before {
		t.Fatalf("main moved: %s -> %s", before, after)
	}
	if _, err := os.Stat(filepath.Join(f.root, "tasks/active/TF-901-example.md")); err != nil {
		t.Fatalf("task left active: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl"))
	if err != nil || !strings.Contains(string(b), `"stage":"rebase"`) {
		t.Fatalf("rebase evidence=%s err=%v", b, err)
	}
}

func configureIntegrationCommand(t *testing.T, root, command string) {
	t.Helper()
	p := filepath.Join(root, ".taskfactory/config.toml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(b), "integration=[\"true\"]", "integration=[\""+command+"\"]", 1)
	if updated == string(b) {
		t.Fatal("integration command config line not found")
	}
	if err = os.WriteFile(p, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", "--", ".taskfactory/config.toml")
	run(t, root, "git", "commit", "-m", "configure integration command")
}

type fixture struct{ root, candidate, result string }

func newFixture(t *testing.T) fixture {
	t.Helper()
	return newFixtureWithGitignore(t, "")
}

// newFixtureWithGitignore commits gitignore as the project .gitignore when it
// is not empty, before the task branch is created.
func newFixtureWithGitignore(t *testing.T, gitignore string) fixture {
	t.Helper()
	root := t.TempDir()
	run(t, root, "git", "init", "-b", "main")
	run(t, root, "git", "config", "user.name", "Test")
	run(t, root, "git", "config", "user.email", "test@example.invalid")
	run(t, root, "git", "config", "commit.gpgsign", "false")
	for _, d := range []string{"tasks/inbox", "tasks/ready", "tasks/active", "tasks/failed", "tasks/archive", ".taskfactory"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	config := "protocol_version = 1\n[workers]\nmax_parallel=4\n[git]\nuse_worktrees=true\nworktree_root=\".taskfactory/worktrees\"\nintegration_strategy=\"ff-only\"\n[integration]\nstop_on_main_failure=true\n[verification]\nworker=[\"true\"]\nintegration=[\"true\"]\n"
	write(t, root, ".taskfactory/config.toml", config)
	baseTask := "# TF-901: Example\n\n## Goal\n\nDo it.\n\n## Dependencies\n\nNone\n\n## Scope\n\nImplement.\n\n## Constraints\n\nKeep it small.\n\n## Success criteria\n\n### C1: works\n\nCheck: true\n\n## Verification\n\nRun it.\n"
	write(t, root, "tasks/ready/TF-901-example.md", baseTask)
	if gitignore != "" {
		write(t, root, ".gitignore", gitignore)
	}
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-m", "initial")
	base, _ := git(root, "rev-parse", "HEAD")
	branch := "task/TF-901"
	run(t, root, "git", "branch", branch, base)
	candidate := filepath.Join(root, ".taskfactory/worktrees/TF-901")
	if err := os.MkdirAll(filepath.Dir(candidate), 0755); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "worktree", "add", candidate, branch)
	active := strings.TrimSuffix(baseTask, "\n") + "\n\n## Claim\n\nOwner: test\nBranch: task/TF-901\nWorktree: " + candidate + "\nBase commit: " + base + "\nStarted at: 2026-10-08T12:00:00Z\n"
	write(t, root, "tasks/active/TF-901-example.md", active)
	if err := os.Remove(filepath.Join(root, "tasks/ready/TF-901-example.md")); err != nil {
		t.Fatal(err)
	}
	write(t, candidate, "feature.txt", "candidate\n")
	run(t, candidate, "git", "add", "feature.txt")
	run(t, candidate, "git", "commit", "-m", "feature")
	result, _ := git(candidate, "rev-parse", "HEAD")
	evidence := `{"task_id":"TF-901","attempt":1,"recorded_at":"2026-10-08T12:00:00Z","outcome":"PASS","branch":"task/TF-901","base_commit":"` + base + `","result_commit":"` + result + `","changed_files":["feature.txt"],"checks":[{"source":"worker","criterion":"","command":"true","exit_code":0,"output":"","error":""}],"note":""}` + "\n"
	write(t, root, ".taskfactory/evidence/TF-901.jsonl", evidence)
	return fixture{root, candidate, result}
}
func write(t *testing.T, root, rel, s string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0644); err != nil {
		t.Fatal(err)
	}
}
func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command(args[0], args[1:]...)
	c.Dir = dir
	if b, e := c.CombinedOutput(); e != nil {
		t.Fatalf("%v: %v\n%s", args, e, b)
	}
}

func writeWorkLogs(t *testing.T, root string) {
	t.Helper()
	write(t, root, ".taskfactory/logs/TF-901/pi-20261008T120000Z.log", "worker output\n")
	write(t, root, ".taskfactory/logs/TF-902/omp-20261008T120000Z.log", "more\n")
}

func TestLogsDirAllowedWithFiles(t *testing.T) {
	f := newFixture(t)
	writeWorkLogs(t, f.root)
	if err := Run(f.root, "TF-901"); err != nil {
		t.Fatalf("Run with logs dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".taskfactory/logs/TF-901/pi-20261008T120000Z.log")); err != nil {
		t.Fatalf("log removed: %v", err)
	}
}

func TestLogsStayUntrackedAndUnstaged(t *testing.T) {
	f := newFixture(t)
	writeWorkLogs(t, f.root)
	if err := Run(f.root, "TF-901"); err != nil {
		t.Fatal(err)
	}
	stat, _ := git(f.root, "show", "--no-renames", "--pretty=format:", "--name-only", "HEAD")
	if strings.Contains(stat, ".taskfactory") || strings.TrimSpace(stat) != "tasks/archive/TF-901-example.md\ntasks/ready/TF-901-example.md" {
		t.Fatalf("integration commit paths: %q", stat)
	}
	tracked, _ := git(f.root, "ls-files", ".taskfactory/logs")
	if strings.TrimSpace(tracked) != "" {
		t.Fatalf("logs tracked: %q", tracked)
	}
	staged, _ := git(f.root, "diff", "--cached", "--name-only")
	if strings.TrimSpace(staged) != "" {
		t.Fatalf("staged: %q", staged)
	}
	status, _ := git(f.root, "status", "--short", "--untracked-files=all", "--", ".taskfactory/logs")
	if !strings.Contains(status, "?? .taskfactory/logs/TF-901/pi-20261008T120000Z.log") {
		t.Fatalf("logs not untracked: %q", status)
	}
}

func TestLogsLookalikeBlocks(t *testing.T) {
	for _, rel := range []string{".taskfactory/logs-old/a.log", ".taskfactory/logsx", "user-file.txt"} {
		t.Run(rel, func(t *testing.T) {
			f := newFixture(t)
			writeWorkLogs(t, f.root)
			write(t, f.root, rel, "x\n")
			before, _ := git(f.root, "rev-parse", "refs/heads/main")
			if err := Run(f.root, "TF-901"); err == nil {
				t.Fatal("Run unexpectedly succeeded")
			}
			after, _ := git(f.root, "rev-parse", "refs/heads/main")
			if after != before {
				t.Fatalf("main moved: %s -> %s", before, after)
			}
		})
	}
}

func TestLogsNoFailedEvidence(t *testing.T) {
	f := newFixture(t)
	writeWorkLogs(t, f.root)
	if err := Run(f.root, "TF-901"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"outcome":"PASS"`) || strings.Contains(string(data), "logs") {
		t.Fatalf("evidence=%s", data)
	}
}

func TestIntegrateWithIgnoredRuntime(t *testing.T) {
	ignore := "# TaskFactory runtime files\n.taskfactory/claim.lock\n.taskfactory/integration.lock\n.taskfactory/evidence/\n.taskfactory/integration-evidence/\n.taskfactory/logs/\n.taskfactory/worktrees/\n"
	f := newFixtureWithGitignore(t, ignore)
	write(t, f.root, ".taskfactory/claim.lock", "")
	writeWorkLogs(t, f.root)
	if err := Run(f.root, "TF-901"); err != nil {
		t.Fatalf("Run with ignored runtime paths: %v", err)
	}
	if err := CheckMain(f.root); err != nil {
		t.Fatalf("CheckMain with ignored runtime paths: %v", err)
	}
	status, _ := git(f.root, "status", "--short")
	if strings.TrimSpace(status) != "" {
		t.Fatalf("status after integrate is not empty: %q", status)
	}
	tracked, _ := git(f.root, "ls-files", ".taskfactory")
	if strings.TrimSpace(tracked) != ".taskfactory/config.toml" {
		t.Fatalf("tracked .taskfactory files: %q", tracked)
	}
}

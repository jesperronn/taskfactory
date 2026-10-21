package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/claim"
	"taskfactory/internal/verify"
)

// stateRepoProject is externalProject with the state directory turned into its
// own git repository that tracks the ready task and the config.
func stateRepoProject(t *testing.T, trackReady bool) (project, state string) {
	t.Helper()
	project, state = externalProject(t)
	run(t, state, "git", "init", "-b", "main")
	run(t, state, "git", "config", "user.name", "State Owner")
	run(t, state, "git", "config", "user.email", "state@example.invalid")
	run(t, state, "git", "config", "commit.gpgsign", "false")
	write(t, state, ".gitignore", ".taskfactory/evidence/\n.taskfactory/integration-evidence/\n.taskfactory/worktrees/\n.taskfactory/*.lock\n")
	run(t, state, "git", "add", ".gitignore", ".taskfactory/config.toml")
	if trackReady {
		run(t, state, "git", "add", "tasks/ready")
	}
	run(t, state, "git", "commit", "-m", "state init")
	return project, state
}

// claimedAndVerified claims TF-901, commits a feature in its worktree and
// verifies it, leaving only integrate to run.
func claimedAndVerified(t *testing.T, project, state string) {
	t.Helper()
	if err := claim.Claim(project, "TF-901", "tester"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	worktree := filepath.Join(state, ".taskfactory/worktrees/TF-901")
	write(t, worktree, "feature.txt", "feature\n")
	run(t, worktree, "git", "add", "feature.txt")
	run(t, worktree, "git", "commit", "-m", "feature")
	if _, err := verify.Verify(project, "TF-901"); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestStateRepoTrackedGuard(t *testing.T) {
	// Tracked in the state repository: the archive commit lands there and the
	// project gets no commit beyond the feature.
	project, state := stateRepoProject(t, true)
	claimedAndVerified(t, project, state)
	before, _ := git(state, "rev-list", "--count", "HEAD")
	if err := Run(project, "TF-901"); err != nil {
		t.Fatalf("integrate: %v", err)
	}
	if subject, _ := git(state, "log", "-1", "--format=%s"); subject != "chore(tasks): archive integrated TF-901" {
		t.Errorf("state HEAD subject = %q", subject)
	}
	if after, _ := git(state, "rev-list", "--count", "HEAD"); after == before {
		t.Errorf("state repository gained no commit")
	}
	if status, _ := git(state, "status", "--porcelain", "--", "tasks"); status != "" {
		t.Errorf("tasks not clean in the state repository: %q", status)
	}
	if subject, _ := git(project, "log", "-1", "--format=%s"); subject != "feature" {
		t.Errorf("project HEAD subject = %q", subject)
	}

	// Untracked in the state repository: the guard refuses, from the state
	// repository's point of view, and the active file is restored.
	project, state = stateRepoProject(t, false)
	claimedAndVerified(t, project, state)
	err := Run(project, "TF-901")
	if err == nil || !strings.Contains(err.Error(), "cannot identify tracked lifecycle deletion") {
		t.Fatalf("integrate err = %v, want the tracked-in-HEAD guard", err)
	}
	if _, e := os.Stat(filepath.Join(state, "tasks/active/TF-901-example.md")); e != nil {
		t.Errorf("active task not restored: %v", e)
	}
	if _, e := os.Stat(filepath.Join(state, "tasks/archive/TF-901-example.md")); !os.IsNotExist(e) {
		t.Errorf("archive file remains: %v", e)
	}
}

func TestStateCommitCrashWindow(t *testing.T) {
	project, state := stateRepoProject(t, true)
	claimedAndVerified(t, project, state)
	hook := filepath.Join(state, ".git/hooks/pre-commit")
	write(t, filepath.Join(state, ".git/hooks"), "pre-commit", "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Run(project, "TF-901"); err == nil {
		t.Fatal("integrate succeeded although the state commit was blocked")
	}
	merged, _ := git(project, "rev-parse", "HEAD")
	if subject, _ := git(project, "log", "-1", "--format=%s", merged); subject != "feature" {
		t.Fatalf("main did not fast-forward: %q", subject)
	}
	evidence, _ := os.ReadFile(filepath.Join(state, ".taskfactory/integration-evidence/TF-901.jsonl"))
	if !strings.Contains(string(evidence), `"outcome":"MERGED"`) {
		t.Fatalf("no merge marker in the state evidence: %s", evidence)
	}
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}

	if err := Run(project, "TF-901"); err != nil {
		t.Fatalf("recovering integrate: %v", err)
	}
	if again, _ := git(project, "rev-parse", "HEAD"); again != merged {
		t.Errorf("project main moved on recovery: %s -> %s", merged, again)
	}
	if count, _ := git(project, "rev-list", "--count", "main"); count != "2" {
		t.Errorf("project has %s commits, want 2 (initial and one feature)", count)
	}
	if subject, _ := git(state, "log", "-1", "--format=%s"); subject != "chore(tasks): archive integrated TF-901" {
		t.Errorf("state HEAD subject = %q", subject)
	}
	if _, e := os.Stat(filepath.Join(state, "tasks/archive/TF-901-example.md")); e != nil {
		t.Errorf("archive missing: %v", e)
	}
	evidence, _ = os.ReadFile(filepath.Join(state, ".taskfactory/integration-evidence/TF-901.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(evidence)), "\n")
	if last := lines[len(lines)-1]; !strings.Contains(last, `"outcome":"PASS"`) || !strings.Contains(last, "Recovered") {
		t.Errorf("last evidence line is not a recovered PASS: %s", last)
	}
}

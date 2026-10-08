package claim

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"taskfactory/internal/taskvalidate"
)

func TestClaimCreatesBranchWorktreeAndValidatedActiveTask(t *testing.T) {
	root := newGitProject(t, 4, "TF-001")
	if err := Claim(root, "TF-001", "worker-a"); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "tasks", "active", "TF-001-target.md")
	data, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Owner: worker-a", "Branch: task/TF-001", "Worktree: " + filepath.Join(realPath(t, root), ".taskfactory", "worktrees", "TF-001"), "Base commit:", "Started at:"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("active metadata missing %q", want)
		}
	}
	if diagnostics := taskvalidate.Validate(root, "tasks/active/TF-001-target.md"); len(diagnostics) != 0 {
		t.Fatalf("active task invalid: %v", diagnostics)
	}
	if _, err := os.Stat(filepath.Join(root, ".taskfactory", "worktrees", "TF-001", "tasks", "ready", "TF-001-target.md")); err != nil {
		t.Fatalf("worktree missing task: %v", err)
	}
	if got := gitOutput(t, root, "worktree", "list", "--porcelain"); !strings.Contains(got, "worktrees/TF-001") {
		t.Fatalf("worktree list lacks claim: %s", got)
	}
}

func TestClaimRejectsEmptyOwnerWithoutChangingTask(t *testing.T) {
	root := newGitProject(t, 4, "TF-001")
	if err := Claim(root, "TF-001", " \t"); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("empty owner error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "ready", "TF-001-target.md")); err != nil {
		t.Fatalf("ready task changed: %v", err)
	}
}

func TestClaimRespectsCapacityAndPreservesExistingBranch(t *testing.T) {
	root := newGitProject(t, 1, "TF-001")
	writeReady(t, root, "TF-002")
	if err := Claim(root, "TF-001", "first"); err != nil {
		t.Fatal(err)
	}
	if err := Claim(root, "TF-002", "second"); err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("capacity error = %v", err)
	}
	branchRoot := newGitProject(t, 4, "TF-003")
	gitRun(t, branchRoot, "branch", "task/TF-003")
	if err := Claim(branchRoot, "TF-003", "third"); err == nil || !strings.Contains(err.Error(), "branch") {
		t.Fatalf("existing branch error = %v", err)
	}
	if got := gitOutput(t, branchRoot, "rev-parse", "task/TF-003"); got != gitOutput(t, branchRoot, "rev-parse", "HEAD") {
		t.Fatalf("preexisting branch changed: %s", got)
	}
	if _, err := os.Stat(filepath.Join(branchRoot, "tasks", "ready", "TF-003-target.md")); err != nil {
		t.Fatalf("ready task changed: %v", err)
	}
}

func TestClaimCreatesIndependentWorktreesForDifferentTasks(t *testing.T) {
	root := newGitProject(t, 2, "TF-001", "TF-002")
	if err := Claim(root, "TF-001", "one"); err != nil {
		t.Fatal(err)
	}
	if err := Claim(root, "TF-002", "two"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"TF-001", "TF-002"} {
		if _, err := os.Stat(filepath.Join(root, ".taskfactory", "worktrees", id)); err != nil {
			t.Fatalf("%s worktree: %v", id, err)
		}
		if diagnostics := taskvalidate.Validate(root, filepath.Join("tasks", "active", id+"-target.md")); len(diagnostics) != 0 {
			t.Fatalf("%s active task invalid: %v", id, diagnostics)
		}
	}
}

func TestClaimGitFailureLeavesReadyTaskAndUnownedResourcesUntouched(t *testing.T) {
	root := newGitProject(t, 4, "TF-001")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	wrapper := "#!/bin/sh\ncase \"$*\" in *\"worktree add\"*) echo injected-worktree-failure >&2; exit 42;; esac\nexec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := Claim(root, "TF-001", "worker"); err == nil || !strings.Contains(err.Error(), "injected-worktree-failure") {
		t.Fatalf("injected Git error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "ready", "TF-001-target.md")); err != nil {
		t.Fatalf("ready task changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".taskfactory", "worktrees", "TF-001")); !os.IsNotExist(err) {
		t.Fatalf("uncreated worktree path was changed: %v", err)
	}
	if _, err := exec.Command(realGit, "-C", root, "show-ref", "--verify", "--quiet", "refs/heads/task/TF-001").Output(); err == nil {
		t.Fatal("failed Git attempt left a branch")
	}
}

func TestClaimFileFailureRollsBackCreatedGitResources(t *testing.T) {
	root := newGitProject(t, 4, "TF-001")
	activeDir := filepath.Join(root, "tasks", "active")
	if err := os.Chmod(activeDir, 0o500); err != nil {
		t.Fatal(err)
	}
	err := Claim(root, "TF-001", "worker")
	_ = os.Chmod(activeDir, 0o755)
	if err == nil || !strings.Contains(err.Error(), "prepare active task file") {
		t.Fatalf("file failure = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "ready", "TF-001-target.md")); err != nil {
		t.Fatalf("ready task changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".taskfactory", "worktrees", "TF-001")); !os.IsNotExist(err) {
		t.Fatalf("created worktree not rolled back: %v", err)
	}
	if _, err := exec.Command("git", "-C", root, "show-ref", "--verify", "--quiet", "refs/heads/task/TF-001").Output(); err == nil {
		t.Fatal("created branch not rolled back")
	}
}

func realPath(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

func TestClaimRejectsOccupiedPathAndGitFailureWithoutUnownedCleanup(t *testing.T) {
	root := newGitProject(t, 4, "TF-001")
	path := filepath.Join(root, ".taskfactory", "worktrees", "TF-001")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(path, "keep")
	if err := os.WriteFile(marker, []byte("owned elsewhere"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Claim(root, "TF-001", "worker"); err == nil || !strings.Contains(err.Error(), "path") {
		t.Fatalf("occupied path error = %v", err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "owned elsewhere" {
		t.Fatalf("occupied path changed: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "ready", "TF-001-target.md")); err != nil {
		t.Fatalf("ready task changed: %v", err)
	}
}

func TestClaimSerializesSameTaskClaims(t *testing.T) {
	root := newGitProject(t, 4, "TF-001")
	results := make(chan error, 2)
	for _, owner := range []string{"one", "two"} {
		go func(owner string) { results <- Claim(root, "TF-001", owner) }(owner)
	}
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("claim results = %v, %v; want one success", first, second)
	}
	if diagnostics := taskvalidate.Validate(root, "tasks/active/TF-001-target.md"); len(diagnostics) != 0 {
		t.Fatalf("active task invalid: %v", diagnostics)
	}
}

func newGitProject(t *testing.T, capacity int, ids ...string) string {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init", "--quiet")
	gitRun(t, root, "config", "user.name", "TaskFactory Test")
	gitRun(t, root, "config", "user.email", "taskfactory@example.invalid")
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		if err := os.MkdirAll(filepath.Join(root, "tasks", state), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, ".taskfactory"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "protocol_version = 1\n[workers]\nmax_parallel = " + string(rune('0'+capacity)) + "\n[git]\nworktree_root = \".taskfactory/worktrees\"\n"
	if err := os.WriteFile(filepath.Join(root, ".taskfactory", "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		writeReady(t, root, id)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "--quiet", "-m", "fixture")
	return root
}

func writeReady(t *testing.T, root, id string) {
	t.Helper()
	content := "# " + id + ": Target\n\n## Goal\n\nDo it.\n\n## Dependencies\n\nNone\n\n## Scope\n\nImplement it.\n\n## Constraints\n\nKeep it small.\n\n## Success criteria\n\n### C1: It works\n\nCheck: go test ./...\n\n## Verification\n\nRun the check.\n"
	if err := os.WriteFile(filepath.Join(root, "tasks", "ready", id+"-target.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(output))
}

var _ = time.RFC3339

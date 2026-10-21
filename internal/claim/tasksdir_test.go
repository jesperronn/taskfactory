package claim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/paths"
	"taskfactory/internal/taskvalidate"
)

// Serial: registers into the process-global paths registry (paths.Register).
func TestTasksDirClaimUsesConfiguredDirectory(t *testing.T) {
	root := newGitProject(t, 4, "TF-001")
	root = realPath(t, root)
	gitRun(t, root, "mv", "tasks", ".taskfactory/tasks")
	gitRun(t, root, "commit", "--quiet", "-m", "move tasks")
	t.Cleanup(paths.Register(paths.Layout{Project: root, State: root, TasksDir: ".taskfactory/tasks"}))

	if err := CheckEligibility(root, "TF-001"); err != nil {
		t.Fatalf("eligibility: %v", err)
	}
	if err := Claim(root, "TF-001", "worker-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks")); !os.IsNotExist(err) {
		t.Fatalf("tasks/ exists: %v", err)
	}
	active := filepath.Join(root, ".taskfactory", "tasks", "active", "TF-001-target.md")
	data, err := os.ReadFile(active)
	if err != nil || !strings.Contains(string(data), "Owner: worker-a") {
		t.Fatalf("active file: %v %s", err, data)
	}
	if _, err := os.Stat(filepath.Join(root, ".taskfactory", "tasks", "ready", "TF-001-target.md")); !os.IsNotExist(err) {
		t.Fatalf("ready file remains: %v", err)
	}
	if got := taskvalidate.Validate(root, ".taskfactory/tasks/active/TF-001-target.md"); len(got) != 0 {
		t.Fatalf("active invalid: %v", got)
	}
	// The worker checkout was made from main and holds the committed contract.
	wt := filepath.Join(root, ".taskfactory", "worktrees", "TF-001", ".taskfactory", "tasks", "ready", "TF-001-target.md")
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("worktree lacks the contract: %v", err)
	}
}

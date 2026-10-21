package fail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/paths"
)

// Serial: registers into the process-global paths registry (paths.Register).
func TestTasksDirLifecycleFail(t *testing.T) {
	root := newProject(t, true)
	runGit(t, root, "mv", "tasks", ".taskfactory/tasks")
	runGit(t, root, "commit", "-q", "-m", "move tasks")
	t.Cleanup(paths.Register(paths.Layout{Project: root, State: root, TasksDir: ".taskfactory/tasks"}))
	writeEvidence(t, root, "FAILED")

	failed, err := Fail(root, "TF-001", Options{Outcome: "FAILED"})
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if failed != ".taskfactory/tasks/failed/TF-001-example.md" {
		t.Fatalf("failed path = %q", failed)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks")); !os.IsNotExist(err) {
		t.Fatalf("tasks/ exists: %v", err)
	}
	names := runGit(t, root, "show", "--no-renames", "--name-status", "--format=", "HEAD")
	if !strings.Contains(names, "D\t.taskfactory/tasks/active/TF-001-example.md") || !strings.Contains(names, "A\t.taskfactory/tasks/failed/TF-001-example.md") || strings.Contains(names, "evidence") {
		t.Fatalf("commit paths = %q", names)
	}
	if _, err := Fail(root, "TF-001", Options{Outcome: "FAILED"}); err == nil || !strings.Contains(err.Error(), ".taskfactory/tasks/") {
		t.Fatalf("second fail error = %v", err)
	}
}

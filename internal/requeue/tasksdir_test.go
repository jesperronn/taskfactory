package requeue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/paths"
)

// Serial: registers into the process-global paths registry (paths.Register).
func TestTasksDirLifecycleRequeue(t *testing.T) {
	root := newProject(t, false)
	runGit(t, root, "mv", "tasks", ".taskfactory/tasks")
	runGit(t, root, "commit", "-q", "-m", "move tasks")
	t.Cleanup(paths.Register(paths.Layout{Project: root, State: root, TasksDir: ".taskfactory/tasks"}))

	res, err := RequeueWith(root, "TF-001", "ready", Options{DryRun: true, Would: func(string, ...any) {}})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if res.Path != ".taskfactory/tasks/ready/"+taskFile {
		t.Fatalf("dry run path = %q", res.Path)
	}
	if status := runGit(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("dry run changed the tree:\n%s", status)
	}
	res, err = Requeue(root, "TF-001", "ready")
	if err != nil {
		t.Fatalf("Requeue: %v", err)
	}
	if res.Path != ".taskfactory/tasks/ready/"+taskFile {
		t.Fatalf("path = %q", res.Path)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks")); !os.IsNotExist(err) {
		t.Fatalf("tasks/ exists: %v", err)
	}
	names := runGit(t, root, "show", "--no-renames", "--name-status", "--format=", "HEAD")
	if !strings.Contains(names, "D\t.taskfactory/tasks/failed/"+taskFile) || !strings.Contains(names, "A\t.taskfactory/tasks/ready/"+taskFile) {
		t.Fatalf("commit paths = %q", names)
	}
}

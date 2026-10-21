package promote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/paths"
)

// Serial: registers into the process-global paths registry (paths.Register).
func TestTasksDirLifecyclePromote(t *testing.T) {
	const name = "TF-010-valid-example.md"
	root := newRepo(t)
	root, _ = filepath.EvalSymlinks(root)
	if err := os.MkdirAll(filepath.Join(root, ".taskfactory"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "mv", "tasks", ".taskfactory/tasks")
	commitAll(t, root, "move tasks")
	t.Cleanup(paths.Register(paths.Layout{Project: root, State: root, TasksDir: ".taskfactory/tasks"}))
	writeFile(t, filepath.Join(root, ".taskfactory", "tasks", "inbox", name), validTask("TF-010", "valid-example"))
	commitAll(t, root, "add proposal")

	rel, err := Promote(root, "TF-010")
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if rel != ".taskfactory/tasks/ready/"+name {
		t.Fatalf("ready path = %q", rel)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks")); !os.IsNotExist(err) {
		t.Fatalf("tasks/ exists: %v", err)
	}
	changed := strings.Join(lines(runGit(t, root, "show", "--no-renames", "--name-only", "--format=", "HEAD")), ",")
	if want := ".taskfactory/tasks/inbox/" + name + ",.taskfactory/tasks/ready/" + name; changed != want {
		t.Fatalf("commit paths = %s, want %s", changed, want)
	}
	if status := runGit(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("tree not clean:\n%s", status)
	}
	// A missing task names the configured directory.
	if _, err := Promote(root, "TF-011"); err == nil || !strings.Contains(err.Error(), ".taskfactory/tasks/inbox") {
		t.Fatalf("error = %v", err)
	}
	// Dry run uses a scratch copy and leaves the project untouched.
	writeFile(t, filepath.Join(root, ".taskfactory", "tasks", "inbox", "TF-012-dry.md"), validTask("TF-012", "dry"))
	commitAll(t, root, "add dry")
	var would []string
	if _, err := PromoteWith(root, "TF-012", Options{DryRun: true, Would: func(f string, a ...any) { would = append(would, fmt.Sprintf(f, a...)) }}); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(would) == 0 || !strings.Contains(would[0], ".taskfactory/tasks/ready") {
		t.Fatalf("would = %v", would)
	}
}

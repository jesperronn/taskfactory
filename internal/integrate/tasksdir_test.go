package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/paths"
)

func TestTasksDirIntegrateArchivesUnderConfiguredDirectory(t *testing.T) {
	f := newFixtureAt(t, "", ".taskfactory/tasks")
	root, _ := filepath.EvalSymlinks(f.root)
	t.Cleanup(paths.Register(paths.Layout{Project: root, State: root, TasksDir: ".taskfactory/tasks"}))
	if err := Run(f.root, "TF-901"); err != nil {
		status, _ := git(f.root, "status", "--short")
		evidence, _ := os.ReadFile(filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl"))
		t.Fatalf("%v; status=%q evidence=%s", err, status, evidence)
	}
	if _, err := os.Stat(filepath.Join(f.root, "tasks")); !os.IsNotExist(err) {
		t.Fatalf("tasks/ exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".taskfactory/tasks/archive/TF-901-example.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".taskfactory/tasks/active/TF-901-example.md")); !os.IsNotExist(err) {
		t.Fatalf("active task remains: %v", err)
	}
	changed, err := git(f.root, "show", "--no-renames", "--pretty=format:", "--name-only", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(changed) != ".taskfactory/tasks/archive/TF-901-example.md\n.taskfactory/tasks/ready/TF-901-example.md" {
		t.Fatalf("archive commit paths: %q", changed)
	}
}

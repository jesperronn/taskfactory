package taskvalidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/paths"
)

// tasksDirProject returns a project whose task folders live in
// .taskfactory/tasks and registers that layout for the test.
func tasksDirProject(t *testing.T) (string, paths.Layout) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	layout := paths.Layout{Project: root, State: root, TasksDir: ".taskfactory/tasks"}
	t.Cleanup(paths.Register(layout))
	for _, state := range paths.States {
		if err := os.MkdirAll(layout.TaskDir(state), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root, layout
}

func TestTasksDirValidateFindsFilesUnderConfiguredDir(t *testing.T) {
	root, layout := tasksDirProject(t)
	if err := os.WriteFile(layout.TaskFile("ready", "TF-101-example.md"), []byte(validTask), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Validate(root, ""); len(got) != 0 {
		t.Fatalf("diagnostics = %#v", got)
	}
	broken := strings.Replace(validTask, "Check: go test ./...", "Check:", 1)
	if err := os.WriteFile(layout.TaskFile("ready", "TF-102-broken.md"), []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	got := Validate(root, ".taskfactory/tasks/ready/TF-102-broken.md")
	if len(got) == 0 || !strings.Contains(got[0].Error(), ".taskfactory/tasks/ready/TF-102-broken.md") {
		t.Fatalf("diagnostics = %#v, want the configured path", got)
	}
}

func TestTasksDirValidateRejectsUnknownDirsWithConfiguredPath(t *testing.T) {
	root, layout := tasksDirProject(t)
	other := filepath.Join(layout.Tasks(), "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "x.md"), []byte("# TF-103: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, ".gitkeep"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got := Validate(root, "")
	if len(got) != 1 || got[0].Error() != ".taskfactory/tasks/other/x.md: state: task files must be under inbox, ready, active, failed, or archive" {
		t.Fatalf("diagnostics = %#v", got)
	}
}

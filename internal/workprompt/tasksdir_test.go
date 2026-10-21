package workprompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/paths"
)

// Serial: registers into the process-global paths registry (paths.Register).
func TestTasksDirClaimBuildsPromptFromConfiguredDirectory(t *testing.T) {
	f := newFixture(t)
	if err := os.Remove(f.taskFile); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(paths.Register(paths.Layout{Project: f.root, State: f.root, TasksDir: ".taskfactory/tasks"}))
	moved := filepath.Join(f.root, ".taskfactory", "tasks", "active", testID+"-example.md")
	write(t, moved, f.task)

	p, err := Build(f.root, testID)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.Contains(p.Text, "----- BEGIN task file .taskfactory/tasks/active/"+testID+"-example.md -----") {
		t.Fatalf("prompt names the wrong contract path:\n%s", p.Text)
	}
	if err := os.Remove(moved); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(f.root, testID); err == nil || !strings.Contains(err.Error(), ".taskfactory/tasks/active") {
		t.Fatalf("missing-task error = %v", err)
	}
}

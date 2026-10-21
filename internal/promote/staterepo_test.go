package promote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/paths"
)

// stateRepo returns a code repository and a separate state repository whose
// inbox tracks TF-010.
func stateRepo(t *testing.T) (project, state string) {
	t.Helper()
	project = newRepo(t)
	project, _ = filepath.EvalSymlinks(project)
	state, _ = filepath.EvalSymlinks(t.TempDir())
	runGit(t, state, "init", "-q", "-b", "main")
	runGit(t, state, "config", "user.name", "State Owner")
	runGit(t, state, "config", "user.email", "state@example.invalid")
	runGit(t, state, "config", "commit.gpgsign", "false")
	for _, d := range paths.States {
		if err := os.MkdirAll(filepath.Join(state, "tasks", d), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(state, "tasks", d, ".gitkeep"), "")
	}
	writeTask(t, state, "inbox", "TF-010-valid-example.md", validTask("TF-010", "valid-example"))
	commitAll(t, state, "state")
	t.Cleanup(paths.Register(paths.Layout{Project: project, State: state}))
	return project, state
}

// Serial: registers into the process-global paths registry (paths.Register).
func TestStateRepoCommit(t *testing.T) {
	project, state := stateRepo(t)
	projectHead := head(t, project)
	if _, err := Promote(project, "TF-010"); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if subject := strings.TrimSpace(runGit(t, state, "log", "-1", "--format=%s")); subject != "docs: promote TF-010 to ready" {
		t.Errorf("state commit subject = %q", subject)
	}
	changed := strings.Fields(runGit(t, state, "show", "--no-renames", "--name-only", "--format=", "HEAD"))
	if strings.Join(changed, ",") != "tasks/inbox/TF-010-valid-example.md,tasks/ready/TF-010-valid-example.md" {
		t.Errorf("state commit paths = %v", changed)
	}
	if status := runGit(t, state, "status", "--porcelain"); status != "" {
		t.Errorf("state tree not clean: %q", status)
	}
	if head(t, project) != projectHead {
		t.Errorf("project HEAD moved")
	}
}

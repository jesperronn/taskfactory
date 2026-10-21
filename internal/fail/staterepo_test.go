package fail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/paths"
)

// externalState returns a code repository (with no task state) and a separate
// state directory holding the claimed TF-001 task. With asRepo the state
// directory is its own git repository that tracks the ready task.
func externalState(t *testing.T, asRepo bool) (project, state string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	project, _ = filepath.EvalSymlinks(t.TempDir())
	state, _ = filepath.EvalSymlinks(t.TempDir())
	runGit(t, project, "init", "-q", "-b", "main")
	runGit(t, project, "config", "user.name", "Test Worker")
	runGit(t, project, "config", "user.email", "worker@example.invalid")
	runGit(t, project, "config", "commit.gpgsign", "false")
	writeFile(t, filepath.Join(project, "README.md"), "code\n")
	runGit(t, project, "add", "--all")
	runGit(t, project, "commit", "-q", "-m", "code")
	for _, d := range paths.States {
		if err := os.MkdirAll(filepath.Join(state, "tasks", d), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(state, "tasks", d, ".gitkeep"), "")
	}
	writeFile(t, filepath.Join(state, ".taskfactory", "config.toml"), "project_root = \""+project+"\"\n"+testConfig)
	writeFile(t, filepath.Join(state, "tasks", "ready", "TF-001-example.md"), taskContract("TF-001", state, false))
	if asRepo {
		runGit(t, state, "init", "-q", "-b", "main")
		runGit(t, state, "config", "user.name", "State Owner")
		runGit(t, state, "config", "user.email", "state@example.invalid")
		runGit(t, state, "config", "commit.gpgsign", "false")
		runGit(t, state, "add", "--all")
		runGit(t, state, "commit", "-q", "-m", "state")
	}
	// A claim is an uncommitted ready-to-active rename.
	if err := os.Rename(filepath.Join(state, "tasks", "ready", "TF-001-example.md"), filepath.Join(state, "tasks", "active", "TF-001-example.md")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(state, "tasks", "active", "TF-001-example.md"), taskContract("TF-001", state, true))
	t.Cleanup(paths.Register(paths.Layout{Project: project, State: state}))
	return project, state
}

func TestPlainStateRename(t *testing.T) {
	project, state := externalState(t, false)
	before := commitCount(t, project)
	rel, err := Fail(project, "TF-001", Options{Outcome: "FAILED", Reason: "stalled", ReasonSet: true})
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if rel != "tasks/failed/TF-001-example.md" {
		t.Errorf("failed path = %q", rel)
	}
	if _, err := os.Stat(filepath.Join(state, rel)); err != nil {
		t.Errorf("failed task missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "tasks/active/TF-001-example.md")); !os.IsNotExist(err) {
		t.Errorf("active task remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, ".git")); !os.IsNotExist(err) {
		t.Errorf("plain state folder gained a repository: %v", err)
	}
	if after := commitCount(t, project); after != before {
		t.Errorf("project commits %s -> %s, want a plain rename", before, after)
	}
}

func TestStateRepoFailCommits(t *testing.T) {
	project, state := externalState(t, true)
	before := commitCount(t, project)
	if _, err := Fail(project, "TF-001", Options{Outcome: "FAILED", Reason: "stalled", ReasonSet: true}); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if body := runGit(t, state, "log", "-1", "--format=%B"); !strings.Contains(body, "docs: mark TF-001 failed") || !strings.Contains(body, "stalled") {
		t.Errorf("state commit message = %q", body)
	}
	if files := runGit(t, state, "show", "--no-renames", "--name-only", "--format=", "HEAD"); !strings.Contains(files, "tasks/ready/TF-001-example.md") || !strings.Contains(files, "tasks/failed/TF-001-example.md") {
		t.Errorf("state commit paths = %q", files)
	}
	if after := commitCount(t, project); after != before {
		t.Errorf("project commits %s -> %s", before, after)
	}
}

package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/claim"
	"taskfactory/internal/paths"
	"taskfactory/internal/verify"
)

// externalProject builds a code repository with no task state in it and a
// separate plain directory (not a git repository) holding tasks/ and
// .taskfactory/config.toml with project_root. It registers the layout for the
// test and returns both roots.
func externalProject(t *testing.T) (project, state string) {
	t.Helper()
	project = t.TempDir()
	state = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(project); err == nil {
		project = resolved
	}
	if resolved, err := filepath.EvalSymlinks(state); err == nil {
		state = resolved
	}
	run(t, project, "git", "init", "-b", "main")
	run(t, project, "git", "config", "user.name", "Test")
	run(t, project, "git", "config", "user.email", "test@example.invalid")
	run(t, project, "git", "config", "commit.gpgsign", "false")
	write(t, project, "README.md", "code\n")
	run(t, project, "git", "add", ".")
	run(t, project, "git", "commit", "-m", "initial")

	for _, d := range []string{"inbox", "ready", "active", "failed", "archive"} {
		if err := os.MkdirAll(filepath.Join(state, "tasks", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config := "project_root = \"" + project + "\"\nprotocol_version = 1\n[workers]\nmax_parallel=4\n[git]\nuse_worktrees=true\nintegration_strategy=\"ff-only\"\n[integration]\nstop_on_main_failure=true\n[verification]\nworker=[\"true\"]\nintegration=[\"true\"]\n"
	write(t, state, ".taskfactory/config.toml", config)
	task := "# TF-901: Example\n\n## Goal\n\nDo it.\n\n## Dependencies\n\nNone\n\n## Scope\n\nImplement.\n\n## Constraints\n\nKeep it small.\n\n## Success criteria\n\n### C1: works\n\nCheck: true\n\n## Verification\n\nRun it.\n"
	write(t, state, "tasks/ready/TF-901-example.md", task)
	t.Cleanup(paths.Register(paths.Layout{Project: project, State: state}))
	return project, state
}

// externalLifecycle claims, works, verifies and integrates TF-901 with state
// outside the project.
func externalLifecycle(t *testing.T) (project, state string) {
	t.Helper()
	project, state = externalProject(t)
	if err := claim.Claim(project, "TF-901", "tester"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "tasks/active/TF-901-example.md")); err != nil {
		t.Fatalf("active task not in the state dir: %v", err)
	}
	worktree := filepath.Join(state, ".taskfactory/worktrees/TF-901")
	write(t, worktree, "feature.txt", "feature\n")
	run(t, worktree, "git", "add", "feature.txt")
	run(t, worktree, "git", "commit", "-m", "feature")
	if _, err := verify.Verify(project, "TF-901"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := Run(project, "TF-901"); err != nil {
		t.Fatalf("integrate: %v", err)
	}
	return project, state
}

// Serial: registers into the process-global paths registry (paths.Register).
func TestExternalStateDir(t *testing.T) {
	project, state := externalLifecycle(t)
	if _, err := os.Stat(filepath.Join(state, "tasks/archive/TF-901-example.md")); err != nil {
		t.Fatalf("archived task missing from the state dir: %v", err)
	}
	for _, rel := range []string{"tasks/active/TF-901-example.md", "tasks/ready/TF-901-example.md"} {
		if _, err := os.Stat(filepath.Join(state, rel)); !os.IsNotExist(err) {
			t.Errorf("%s remains: %v", rel, err)
		}
	}
	for _, rel := range []string{".taskfactory/evidence/TF-901.jsonl", ".taskfactory/integration-evidence/TF-901.jsonl"} {
		data, err := os.ReadFile(filepath.Join(state, rel))
		if err != nil || !strings.Contains(string(data), `"outcome":"PASS"`) {
			t.Errorf("%s = %q, %v", rel, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(state, ".git")); !os.IsNotExist(err) {
		t.Errorf("the state dir must stay a plain folder: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(project, "feature.txt")); err != nil || string(data) != "feature\n" {
		t.Errorf("main did not fast-forward to the feature: %q, %v", data, err)
	}
	subject, _ := git(project, "log", "-1", "--format=%s")
	if subject != "feature" {
		t.Errorf("project HEAD subject = %q, want the feature commit with no archive commit after it", subject)
	}
}

// Serial: registers into the process-global paths registry (paths.Register).
func TestExternalStateCleanProject(t *testing.T) {
	project, _ := externalLifecycle(t)
	status, err := git(project, "status", "--porcelain", "--untracked-files=all")
	if err != nil || status != "" {
		t.Errorf("project tree is not clean: %q, %v", status, err)
	}
	for _, rel := range []string{"tasks", ".taskfactory", ".gitignore"} {
		if _, err := os.Lstat(filepath.Join(project, rel)); !os.IsNotExist(err) {
			t.Errorf("project contains %s: %v", rel, err)
		}
	}
	if count, _ := git(project, "rev-list", "--count", "main"); count != "2" {
		t.Errorf("project has %s commits, want 2 (initial and feature)", count)
	}
}

// Serial: registers into the process-global paths registry (paths.Register).
func TestExternalStateRequiresProjectRoot(t *testing.T) {
	project, state := externalProject(t)
	write(t, state, ".taskfactory/config.toml", "protocol_version = 1\n")
	err := claim.Claim(project, "TF-901", "tester")
	if err == nil || !strings.Contains(err.Error(), "project_root") {
		t.Fatalf("claim err = %v, want an error naming project_root", err)
	}
}

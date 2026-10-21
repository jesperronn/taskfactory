package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const tasksDirConfig = "tasks_dir = \".taskfactory/tasks\"\n\n[verification]\nworker = [\"true\"]\nintegration = [\"true\"]\n"

var tasksDirEnv = []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull}

// tasksDirProject is a git project, identity and signing set in its own
// config, whose config.toml sets tasks_dir under .taskfactory.
func tasksDirProject(t *testing.T) string {
	t.Helper()
	root := initGitProject(t)
	mustGit(t, root, "config", "user.name", "Test Worker")
	mustGit(t, root, "config", "user.email", "worker@example.invalid")
	mustGit(t, root, "config", "commit.gpgsign", "false")
	if err := os.MkdirAll(filepath.Join(root, ".taskfactory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskfactory", "config.toml"), []byte(tasksDirConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), tasksDirEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestTasksDirInitKeepsKeyAndCreatesFoldersThere(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := tasksDirProject(t)
	for i := 1; i <= 2; i++ {
		if res := runWithEnv(t, binary, root, tasksDirEnv, "init"); res.code != 0 {
			t.Fatalf("init run %d: %+v", i, res)
		}
	}
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		if info, err := os.Stat(filepath.Join(root, ".taskfactory", "tasks", state)); err != nil || !info.IsDir() {
			t.Errorf("missing .taskfactory/tasks/%s: %v", state, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "tasks")); !os.IsNotExist(err) {
		t.Errorf("init created tasks/: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".taskfactory", "config.toml"))
	if err != nil || string(got) != tasksDirConfig {
		t.Fatalf("config changed: %q %v", got, err)
	}
	// A new project writes no tasks_dir.
	plain := initGitProject(t)
	if res := runWithEnv(t, binary, plain, tasksDirEnv, "init"); res.code != 0 {
		t.Fatalf("plain init: %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(plain, ".taskfactory", "config.toml"))
	if strings.Contains(string(data), "tasks_dir") {
		t.Fatalf("default init wrote tasks_dir:\n%s", data)
	}
}

func TestTasksDirIgnoreLeavesTaskFoldersTrackable(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := tasksDirProject(t)
	if res := runWithEnv(t, binary, root, tasksDirEnv, "init"); res.code != 0 {
		t.Fatalf("init: %+v", res)
	}
	got, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if string(got) != wantGitignoreBlock {
		t.Fatalf(".gitignore = %q", got)
	}
	for _, p := range []string{".taskfactory/tasks", ".taskfactory/tasks/ready/TF-001-x.md", ".taskfactory/tasks/archive/TF-001-x.md", ".taskfactory/config.toml"} {
		cmd := exec.Command("git", "-C", root, "check-ignore", "-q", p)
		if err := cmd.Run(); processExitCode(err) != 1 {
			t.Errorf("%s must not be ignored: %v", p, err)
		}
	}
	for _, p := range []string{".taskfactory/evidence/TF-001.jsonl", ".taskfactory/worktrees/TF-001", ".taskfactory/logs/TF-001/a.log"} {
		if err := exec.Command("git", "-C", root, "check-ignore", "-q", p).Run(); err != nil {
			t.Errorf("%s must be ignored: %v", p, err)
		}
	}
	// An older project's .gitignore, already complete or with its own lines, is
	// not rewritten by init.
	for _, old := range []string{wantGitignoreBlock, "build/\n" + wantGitignoreBlock, "# mine\n.taskfactory/*\n!.taskfactory/tasks/\n" + wantGitignoreBlock} {
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(old), 0o644); err != nil {
			t.Fatal(err)
		}
		if res := runWithEnv(t, binary, root, tasksDirEnv, "init"); res.code != 0 {
			t.Fatalf("init: %+v", res)
		}
		after, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
		if string(after) != old {
			t.Errorf(".gitignore rewritten:\n%q\nwas\n%q", after, old)
		}
	}
}

func TestTasksDirCompleteListsIDsUnderConfiguredDirectory(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := tasksDirProject(t)
	files := map[string][]string{
		"inbox":  {"TF-010-inbox.md"},
		"ready":  {"TF-002-b.md", "TF-001-a.md"},
		"active": {"TF-003-c.md"},
	}
	for state, names := range files {
		dir := filepath.Join(root, ".taskfactory", "tasks", state)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, n := range names {
			if err := os.WriteFile(filepath.Join(dir, n), []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for command, want := range map[string]string{"promote": "TF-010", "claim": "TF-001\nTF-002", "fail": "TF-003"} {
		res := runSplit(t, binary, root, "__complete", "ids", command)
		if res.code != 0 || res.stderr != "" || strings.TrimSpace(res.stdout) != want {
			t.Errorf("__complete ids %s = %+v, want %q", command, res, want)
		}
	}
}

func TestTasksDirValidateArguments(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := tasksDirProject(t)
	if res := runWithEnv(t, binary, root, tasksDirEnv, "init"); res.code != 0 {
		t.Fatalf("init: %+v", res)
	}
	base := filepath.Join(root, ".taskfactory", "tasks")
	write := func(rel, content string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(base, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("ready/TF-101-ok.md", validTask("TF-101"))
	write("archive/TF-102-old.md", "# TF-102: Old\n\nlegacy\n")
	for _, args := range [][]string{{"validate"}, {"validate", ".taskfactory/tasks"}, {"validate", ".taskfactory/tasks/ready"}, {"validate", ".taskfactory/tasks/ready/TF-101-ok.md"}} {
		if res := runWithEnv(t, binary, root, tasksDirEnv, args...); res.code != 0 {
			t.Errorf("%v = %+v", args, res)
		}
	}
	// The archive is read for IDs only when the configured directory is named.
	if res := runWithEnv(t, binary, root, tasksDirEnv, "validate", ".taskfactory/tasks/archive"); res.code == 0 {
		t.Errorf("validating the archive by name should check the legacy file: %+v", res)
	}
	write("other/x.md", "# TF-103: x\n")
	res := runWithEnv(t, binary, root, tasksDirEnv, "validate", ".taskfactory/tasks")
	if res.code == 0 || !strings.Contains(res.stderr, ".taskfactory/tasks/other/x.md: state") {
		t.Errorf("unknown dir = %+v", res)
	}
	res = runWithEnv(t, binary, root, tasksDirEnv, "validate", "tasks")
	if res.code != 2 || !strings.Contains(res.stderr, ".taskfactory/tasks") {
		t.Errorf("outside path = %+v", res)
	}
}

func TestTasksDirEndToEnd(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := tasksDirProject(t)
	run := func(args ...string) cliResult {
		t.Helper()
		res := runWithEnv(t, binary, root, tasksDirEnv, args...)
		if res.code != 0 {
			t.Fatalf("taskfactory %v: %+v", args, res)
		}
		return res
	}
	run("init")
	if err := os.WriteFile(filepath.Join(root, ".taskfactory", "tasks", "inbox", "TF-901-example.md"), []byte(strings.Replace(validTask("TF-901"), "Check: go test ./...", "Check: true", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", "-A")
	gitOut(t, root, "commit", "-q", "-m", "fixture")
	base := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))

	run("promote", "TF-901")
	run("claim", "TF-901", "--owner", "e2e")
	worktree := filepath.Join(root, ".taskfactory", "worktrees", "TF-901")
	if err := os.WriteFile(filepath.Join(worktree, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, worktree, "add", "feature.txt")
	gitOut(t, worktree, "commit", "-q", "-m", "feat: example")
	run("verify", "TF-901")
	run("integrate", "TF-901")

	if _, err := os.Stat(filepath.Join(root, "tasks")); !os.IsNotExist(err) {
		t.Errorf("tasks/ exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".taskfactory", "tasks", "archive", "TF-901-example.md")); err != nil {
		t.Errorf("archived task missing: %v", err)
	}
	log := gitOut(t, root, "log", "--stat", "--format=commit %s", base+"..HEAD")
	t.Logf("git log --stat %s..HEAD\n%s", base[:7], log)
	commits := strings.Fields(strings.TrimSpace(gitOut(t, root, "rev-list", base+"..HEAD")))
	if len(commits) != 3 {
		t.Fatalf("want promote, feature and archive commits, got %d:\n%s", len(commits), log)
	}
	for _, c := range commits {
		subject := strings.TrimSpace(gitOut(t, root, "show", "-s", "--format=%s", c))
		for _, p := range strings.Fields(gitOut(t, root, "show", "--no-renames", "--name-only", "--format=", c)) {
			if p == "feature.txt" && subject == "feat: example" {
				continue
			}
			if !strings.HasPrefix(p, ".taskfactory/tasks/") {
				t.Errorf("commit %q touches %s outside .taskfactory/tasks", subject, p)
			}
		}
	}
	if status := gitOut(t, root, "status", "--short"); status != "" {
		t.Errorf("git status --short after integrate:\n%s", status)
	}
	for _, p := range []string{".taskfactory/evidence/TF-901.jsonl", ".taskfactory/integration-evidence/TF-901.jsonl", ".taskfactory/claim.lock", ".taskfactory/integration.lock", ".taskfactory/logs/x", ".taskfactory/worktrees/TF-901"} {
		if err := exec.Command("git", "-C", root, "check-ignore", "-q", p).Run(); err != nil {
			t.Errorf("%s is not ignored: %v", p, err)
		}
	}
	gitOut(t, root, "ls-files", "--error-unmatch", ".taskfactory/config.toml")
	if err := exec.Command("git", "-C", root, "diff", "--quiet", base, "HEAD", "--", ".taskfactory/config.toml").Run(); err != nil {
		t.Errorf("config.toml changed since the fixture commit: %v", err)
	}
}

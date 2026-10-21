package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func initPathExists(t *testing.T, dir string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, ".taskfactory", "config.toml"))
	return err == nil
}

func TestInitPathDefaultCwd(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	out, err := runCLI(t, binary, root, "init")
	if processExitCode(err) != 0 || !strings.HasPrefix(string(out), "Next steps:\n") {
		t.Fatalf("exit=%d output=%q", processExitCode(err), out)
	}
	if !initPathExists(t, root) {
		t.Fatal("config not written in cwd repository")
	}
}

func TestInitPathRepoRoot(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	elsewhere := t.TempDir()
	out, err := runCLI(t, binary, elsewhere, "init", root)
	if processExitCode(err) != 0 || !strings.HasPrefix(string(out), "Next steps:\n") {
		t.Fatalf("exit=%d output=%q", processExitCode(err), out)
	}
	if !initPathExists(t, root) || initPathExists(t, elsewhere) {
		t.Fatal("expected config only in target root")
	}
}

func TestInitPathSubdirectory(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, binary, t.TempDir(), "init", sub)
	if processExitCode(err) != 0 || !strings.HasPrefix(string(out), "Next steps:\n") {
		t.Fatalf("exit=%d output=%q", processExitCode(err), out)
	}
	if !initPathExists(t, root) || initPathExists(t, sub) {
		t.Fatal("expected config at repository root only")
	}
}

func TestInitPathRelativeAndSymlink(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	rootLink := filepath.Join(base, "rootlink")
	subLink := filepath.Join(base, "sublink")
	if err := os.Symlink(root, rootLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sub, subLink); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(base, sub)
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{rel, "rootlink", "sublink"} {
		if err := os.RemoveAll(filepath.Join(root, ".taskfactory")); err != nil {
			t.Fatal(err)
		}
		out, err := runCLI(t, binary, base, "init", arg)
		if processExitCode(err) != 0 || !strings.HasPrefix(string(out), "Next steps:\n") {
			t.Fatalf("%s: exit=%d output=%q", arg, processExitCode(err), out)
		}
		if !initPathExists(t, root) || initPathExists(t, sub) {
			t.Fatalf("%s: expected config at repository root only", arg)
		}
	}
}

func TestInitPathNotADirectory(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	cwd := initGitProject(t)
	file := filepath.Join(cwd, "plain.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ arg, want string }{
		{"no-such-dir", "does not exist"},
		{"plain.txt", "not a directory"},
	}
	for _, tc := range cases {
		out, err := runCLI(t, binary, cwd, "init", tc.arg)
		if processExitCode(err) != 1 || !strings.Contains(string(out), tc.arg) || !strings.Contains(string(out), tc.want) {
			t.Errorf("%s: exit=%d output=%q", tc.arg, processExitCode(err), out)
		}
	}
	if initPathExists(t, cwd) {
		t.Fatal("failed init wrote files")
	}
}

func TestInitPathNotInRepo(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	outside := t.TempDir()
	out, err := runCLI(t, binary, initGitProject(t), "init", outside)
	if processExitCode(err) != 1 || !strings.Contains(string(out), outside) || !strings.Contains(string(out), "not inside a Git working tree") {
		t.Fatalf("exit=%d output=%q", processExitCode(err), out)
	}
	if _, statErr := os.Stat(filepath.Join(outside, ".taskfactory")); !os.IsNotExist(statErr) {
		t.Fatal("wrote outside repository")
	}
}

func TestInitPathTwoPositionals(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	out, err := runCLI(t, binary, root, "init", root, "extra-arg")
	if processExitCode(err) != 2 || !strings.Contains(string(out), "extra-arg") {
		t.Fatalf("exit=%d output=%q", processExitCode(err), out)
	}
	if initPathExists(t, root) {
		t.Fatal("usage error wrote files")
	}
}

func TestInitPathHelp(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	for _, args := range [][]string{{"init", "--help"}, {"init", "-h"}, {"init", root, "--help"}, {"init", root, "-h"}, {"init", "--help", root}} {
		out, err := runCLI(t, binary, t.TempDir(), args...)
		if processExitCode(err) != 0 || !strings.HasPrefix(string(out), "Usage: taskfactory init [path]") || !strings.Contains(string(out), "path") {
			t.Errorf("%v: exit=%d output=%q", args, processExitCode(err), out)
		}
	}
	if initPathExists(t, root) {
		t.Fatal("help wrote files")
	}
	out, err := runCLI(t, binary, root, "--help")
	if processExitCode(err) != 0 || !strings.Contains(string(out), "init [path]") {
		t.Fatalf("top-level help: %q", out)
	}
	var initLine, statusLine string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "init [path]") {
			initLine = line
		}
		if strings.Contains(line, "show tasks by state") {
			statusLine = line
		}
	}
	if strings.Index(initLine, "initialize") != strings.Index(statusLine, "show tasks") {
		t.Errorf("descriptions misaligned:\n%s\n%s", initLine, statusLine)
	}
}

func TestInitPathNoWritesToCwdRepo(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	cwd := initGitProject(t)
	target := initGitProject(t)
	out, err := runCLI(t, binary, cwd, "init", target)
	if processExitCode(err) != 0 || !strings.HasPrefix(string(out), "Next steps:\n") {
		t.Fatalf("exit=%d output=%q", processExitCode(err), out)
	}
	entries, err := os.ReadDir(cwd)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != ".git" {
			t.Errorf("cwd repository was written: %s", e.Name())
		}
	}
	if !initPathExists(t, target) {
		t.Fatal("target not initialized")
	}
}

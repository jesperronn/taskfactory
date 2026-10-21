package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveStateDirOrder(t *testing.T) {
	t.Parallel()
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "TF_STATE_DIR" {
				return v
			}
			return ""
		}
	}
	for _, c := range []struct {
		flag, env, dir, source string
	}{
		{"/flag", "/env", "/flag", "--state-dir"},
		{"/flag", "", "/flag", "--state-dir"},
		{"", "/env", "/env", "TF_STATE_DIR"},
		{"", "", "", "default"},
	} {
		dir, source := resolveStateDir(c.flag, env(c.env))
		if dir != c.dir || source != c.source {
			t.Errorf("resolveStateDir(%q, env %q) = %q, %q; want %q, %q", c.flag, c.env, dir, source, c.dir, c.source)
		}
	}
}

func TestExtractStateDirFlag(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"--state-dir", "/x", "TF-001"},
		{"TF-001", "--state-dir=/x"},
	} {
		rest, flags, err := extractCommonFlags(args)
		if err != nil || flags.StateDir != "/x" || strings.Join(rest, " ") != "TF-001" {
			t.Errorf("extractCommonFlags(%v) = %v, %+v, %v", args, rest, flags, err)
		}
	}
	for _, args := range [][]string{{"--state-dir"}, {"--state-dir="}} {
		if _, _, err := extractCommonFlags(args); err == nil {
			t.Errorf("extractCommonFlags(%v) accepted a missing value", args)
		}
	}
}

// externalStateDir returns a plain state directory whose config names a fresh
// code repository and whose inbox holds n loosely valid tasks.
func externalStateDir(t *testing.T, n int, withProjectRoot bool) string {
	t.Helper()
	project := initGitProject(t)
	state := t.TempDir()
	config := "protocol_version = 1\n"
	if withProjectRoot {
		config = fmt.Sprintf("project_root = %q\n", project) + config
	}
	if err := os.MkdirAll(filepath.Join(state, ".taskfactory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, ".taskfactory", "config.toml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"inbox", "ready", "active", "failed", "archive"} {
		if err := os.MkdirAll(filepath.Join(state, "tasks", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= n; i++ {
		name := fmt.Sprintf("TF-%03d-task.md", i)
		if err := os.WriteFile(filepath.Join(state, "tasks", "inbox", name), []byte(fmt.Sprintf("# TF-%03d: task\n", i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return state
}

func runStateEnv(t *testing.T, binary, dir string, env []string, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	cmd.Env = append(cmd.Env, env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return cliResult{stdout.String(), stderr.String(), processExitCode(err)}
}

func TestStateDirDiscovery(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	byFlag := externalStateDir(t, 1, true)
	byEnv := externalStateDir(t, 2, true)
	elsewhere := t.TempDir() // not a git repository: the cwd must not matter
	inbox := func(res cliResult) string {
		for _, line := range strings.Split(res.stdout, "\n") {
			if strings.HasPrefix(line, "inbox: ") {
				return strings.TrimPrefix(line, "inbox: ")
			}
		}
		return "?"
	}

	res := runStateEnv(t, binary, elsewhere, []string{"TF_STATE_DIR=" + byEnv}, "status", "--state-dir", byFlag)
	if res.code != 0 || inbox(res) != "1" {
		t.Errorf("flag and env: %+v, want the flag's state (inbox: 1)", res)
	}
	res = runStateEnv(t, binary, elsewhere, []string{"TF_STATE_DIR=" + byEnv}, "status")
	if res.code != 0 || inbox(res) != "2" {
		t.Errorf("env only: %+v, want the env state (inbox: 2)", res)
	}

	// With neither, the default layout is the project the command runs in.
	project := initGitProject(t)
	if res := runStateEnv(t, binary, project, []string{"TF_STATE_DIR="}, "init"); res.code != 0 {
		t.Fatalf("init: %+v", res)
	}
	res = runStateEnv(t, binary, project, []string{"TF_STATE_DIR="}, "status")
	if res.code != 0 || inbox(res) != "0" {
		t.Errorf("default: %+v, want the project's own state (inbox: 0)", res)
	}

	// An external state directory without project_root is refused by name.
	missing := externalStateDir(t, 0, false)
	res = runStateEnv(t, binary, elsewhere, nil, "status", "--state-dir="+missing)
	if res.code != 1 || !strings.Contains(res.stderr, "project_root") {
		t.Errorf("missing project_root: %+v, want exit 1 naming project_root", res)
	}

	// skill does not take the flag (init does, since TF-103).
	res = runStateEnv(t, binary, project, nil, "skill", "--state-dir", byFlag)
	if res.code != exitUsage {
		t.Errorf("skill --state-dir: %+v, want exit %d", res, exitUsage)
	}
}

// Serial: sets process-global environment or working directory, which is unsafe in a parallel test.
func TestInitStateDir(t *testing.T) {
	binary := buildCLI(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	project := initGitProject(t)
	resolved, err := filepath.EvalSymlinks(project)
	if err != nil {
		t.Fatal(err)
	}

	for _, asRepo := range []bool{false, true} {
		state := filepath.Join(t.TempDir(), "state")
		if err := os.MkdirAll(state, 0o755); err != nil {
			t.Fatal(err)
		}
		if asRepo {
			if out, err := exec.Command("git", "-C", state, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v %s", err, out)
			}
		}
		res := runStateEnv(t, binary, project, []string{"TF_STATE_DIR="}, "init", "--state-dir", state)
		if res.code != 0 {
			t.Fatalf("init --state-dir (repo=%v): %+v", asRepo, res)
		}
		data, err := os.ReadFile(filepath.Join(state, ".taskfactory", "config.toml"))
		if err != nil || !strings.Contains(string(data), fmt.Sprintf("project_root = %q", resolved)) || !strings.Contains(string(data), "protocol_version = 1") {
			t.Errorf("state config (repo=%v) = %q, %v", asRepo, data, err)
		}
		for _, d := range []string{"inbox", "ready", "active", "failed", "archive"} {
			if info, err := os.Stat(filepath.Join(state, "tasks", d)); err != nil || !info.IsDir() {
				t.Errorf("tasks/%s missing in the state dir (repo=%v): %v", d, asRepo, err)
			}
		}
		ignore, ignoreErr := os.ReadFile(filepath.Join(state, ".gitignore"))
		if asRepo != (ignoreErr == nil) || (asRepo && !strings.Contains(string(ignore), ".taskfactory/evidence/")) {
			t.Errorf(".gitignore in the state dir (repo=%v) = %q, %v", asRepo, ignore, ignoreErr)
		}
		// Nothing in the project, and the state is usable by later commands.
		for _, rel := range []string{"tasks", ".taskfactory", ".gitignore"} {
			if _, err := os.Lstat(filepath.Join(project, rel)); !os.IsNotExist(err) {
				t.Errorf("project contains %s: %v", rel, err)
			}
		}
		if res := runStateEnv(t, binary, t.TempDir(), nil, "status", "--state-dir", state); res.code != 0 || !strings.Contains(res.stdout, "inbox: 0") {
			t.Errorf("status on the new state dir (repo=%v): %+v", asRepo, res)
		}
	}
	if !strings.Contains(commandHelps["init"], "--state-dir") {
		t.Error("init help does not document --state-dir")
	}
}

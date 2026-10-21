package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/skills"
)

// skillEnv returns a project and a temporary HOME, never the real ones.
func skillEnv(t *testing.T) (binary, root string, env []string) {
	t.Helper()
	binary = buildCLI(t)
	root = initGitProject(t)
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return binary, root, []string{"HOME=" + t.TempDir()}
}

func TestSkillInstallDryRun(t *testing.T) {
	binary, root, env := skillEnv(t)
	res := runWithEnv(t, binary, root, env, "skill", "install", "--dry-run")
	if res.code != 0 {
		t.Fatalf("%+v", res)
	}
	for _, want := range []string{"would: create directory ", "would: write file ", "SKILL.md", ".taskfactory-skill"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("missing %q in %q", want, res.stdout)
		}
	}
	if !strings.HasSuffix(res.stdout, "dry run: nothing changed\n") || strings.Contains(res.stdout, "next:") {
		t.Errorf("stdout %q", res.stdout)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents")); err == nil {
		t.Error("dry run created .agents")
	}
}

func TestSkillStatus(t *testing.T) {
	binary, root, env := skillEnv(t)
	res := runWithEnv(t, binary, root, env, "skill", "status")
	dir := filepath.Join(root, ".agents", "skills", "taskfactory")
	if res.code != 1 || !strings.Contains(res.stdout, "checked: "+filepath.Join(dir, "SKILL.md")) || !strings.Contains(res.stdout, "missing\n") {
		t.Fatalf("missing: %+v", res)
	}
	if !strings.Contains(res.stderr, "next: taskfactory skill install") {
		t.Errorf("no failure hint: %q", res.stderr)
	}
	res = runWithEnv(t, binary, root, env, "skill", "install", "--verbose")
	if res.code != 0 || !strings.Contains(res.stdout, "next: taskfactory skill status") || !strings.Contains(res.stderr, "verbose: write "+filepath.Join(dir, "SKILL.md")) {
		t.Fatalf("install: %+v", res)
	}
	got, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil || string(got) != skills.SkillMD {
		t.Fatalf("installed file differs: %v", err)
	}
	res = runWithEnv(t, binary, root, env, "skill", "status")
	if res.code != 0 || !strings.HasSuffix(res.stdout, "present\n") {
		t.Fatalf("present: %+v", res)
	}
	res = runWithEnv(t, binary, root, env, "skill", "install")
	if res.code != 1 || !strings.Contains(res.stderr, dir) || !strings.Contains(res.stderr, "--force") {
		t.Fatalf("refuse: %+v", res)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res = runWithEnv(t, binary, root, env, "skill", "status")
	if res.code != 1 || !strings.Contains(res.stdout, "stale: installed ") {
		t.Fatalf("stale: %+v", res)
	}
	if res = runWithEnv(t, binary, root, env, "skill", "install", "--force"); res.code != 0 {
		t.Fatalf("force: %+v", res)
	}
	if res = runWithEnv(t, binary, root, env, "skill", "status"); res.code != 0 {
		t.Fatalf("after force: %+v", res)
	}
}

func TestSkillGlobalAndOutsideProject(t *testing.T) {
	binary := buildCLI(t)
	home := t.TempDir()
	env := []string{"HOME=" + home}
	outside := t.TempDir()
	if res := runWithEnv(t, binary, outside, env, "skill", "install"); res.code != 1 || !strings.Contains(res.stderr, "project") {
		t.Fatalf("outside: %+v", res)
	}
	if res := runWithEnv(t, binary, outside, env, "skill", "install", "--global"); res.code != 0 {
		t.Fatalf("global: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "taskfactory", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(outside, "custom")
	if res := runWithEnv(t, binary, outside, env, "skill", "install", "--target", "dir:"+custom); res.code != 0 {
		t.Fatalf("dir: %+v", res)
	}
	if res := runWithEnv(t, binary, outside, env, "skill", "status", "--target=dir:"+custom); res.code != 0 {
		t.Fatalf("dir status: %+v", res)
	}
}

func TestSkillUsageErrors(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	env := []string{"HOME=" + t.TempDir()}
	for _, args := range [][]string{
		{"skill"},
		{"skill", "bogus"},
		{"skill", "install", "--bogus"},
		{"skill", "install", "extra"},
		{"skill", "status", "--force"},
		{"skill", "install", "--target"},
		{"skill", "install", "--target", "nope"},
		{"skill", "install", "--global", "--target", "dir:/x"},
	} {
		res := runWithEnv(t, binary, dir, env, args...)
		if res.code != 2 || !strings.Contains(res.stderr, "taskfactory skill: ") || res.stdout != "" {
			t.Errorf("%v: %+v", args, res)
		}
	}
}

func TestSkillHelpAndHints(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	env := []string{"HOME=" + t.TempDir()}
	for args, want := range map[string]string{
		"skill --help":         "Usage: taskfactory skill <install|status>",
		"skill install --help": "Usage: taskfactory skill install",
		"skill status --help":  "Usage: taskfactory skill status",
		"--help":               "skill <install|status>",
	} {
		res := runWithEnv(t, binary, dir, env, strings.Fields(args)...)
		if res.code != 0 || !strings.Contains(res.stdout, want) {
			t.Errorf("%s: %+v", args, res)
		}
	}
	if h := nextSteps["skill-install"]; h.success != "taskfactory skill status" || h.failure != "rerun with --force to replace it, or choose --target" {
		t.Errorf("skill-install hint %+v", h)
	}
	if h := nextSteps["skill-status"]; h.failure != "taskfactory skill install" {
		t.Errorf("skill-status hint %+v", h)
	}
}

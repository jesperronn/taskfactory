package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPromoteHelpAndExitCodesMatchOtherCommands(t *testing.T) {
	binary := buildCLI(t)
	outside := t.TempDir()

	help, err := runCLI(t, binary, outside, "promote", "--help")
	if got := processExitCode(err); got != 0 {
		t.Fatalf("promote --help exit = %d, want 0; output: %s", got, help)
	}
	for _, want := range []string{"Usage: taskfactory promote <ID>", "Exit codes:", "--help, -h", "  2  invalid usage"} {
		if !strings.Contains(string(help), want) {
			t.Errorf("promote --help does not contain %q: %s", want, help)
		}
	}

	top, err := runCLI(t, binary, outside, "--help")
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, top)
	}
	if !strings.Contains(string(top), "promote <ID>") || !strings.Contains(string(top), "move an inbox task to ready") {
		t.Errorf("top-level help does not list promote: %s", top)
	}

	for _, args := range [][]string{{"promote"}, {"promote", "TF-001", "extra"}} {
		out, err := runCLI(t, binary, outside, args...)
		if got := processExitCode(err); got != 2 {
			t.Errorf("%v exit = %d, want 2; output: %s", args, got, out)
		}
	}

	out, err := runCLI(t, binary, outside, "promote", "TF-001")
	if got := processExitCode(err); got != 1 {
		t.Errorf("promote outside a project exit = %d, want 1; output: %s", got, out)
	}

	project := initGitProject(t)
	if err := os.MkdirAll(project+"/tasks/inbox", 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", project, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "--allow-empty", "-m", "fixture").CombinedOutput(); err != nil {
		t.Fatalf("fixture commit: %v\n%s", err, out)
	}
	out, err = runCLI(t, binary, project, "promote", "TF-999")
	if got := processExitCode(err); got != 1 {
		t.Errorf("promote unknown ID exit = %d, want 1; output: %s", got, out)
	}
	if !strings.Contains(string(out), "no task file exists in tasks/inbox") {
		t.Errorf("promote unknown ID output = %s", out)
	}
	out, err = runCLI(t, binary, project, "promote", "bad-id")
	if got := processExitCode(err); got != 2 {
		t.Errorf("promote invalid ID exit = %d, want 2; output: %s", got, out)
	}
}

func TestPromoteNamesDependenciesRule(t *testing.T) {
	binary := buildCLI(t)
	project := initGitProject(t)
	if out, err := runCLI(t, binary, project, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	task := "# TF-001: Example\n\n## Goal\n\nDo it.\n\n## Dependencies\n\nNone.\n\n## Scope\n\nImplement.\n\n## Constraints\n\nKeep it small.\n\n## Success criteria\n\n### C1: It works\n\nCheck: go test ./...\n\n## Verification\n\nRun check.\n"
	if err := os.WriteFile(project+"/tasks/inbox/TF-001-example.md", []byte(task), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", project, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", project, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "-m", "fixture").CombinedOutput(); err != nil {
		t.Fatalf("fixture commit: %v\n%s", err, out)
	}
	// Inbox validation is loose: the file passes validate, promote refuses.
	if out, err := runCLI(t, binary, project, "validate", "tasks/inbox/TF-001-example.md"); err != nil {
		t.Fatalf("validate of loose inbox file: %v\n%s", err, out)
	}
	out, err := runCLI(t, binary, project, "promote", "TF-001")
	if processExitCode(err) != 1 || !strings.Contains(string(out), "Dependencies: use None or - TF-NNN lines") {
		t.Fatalf("promote should name the Dependencies rule: exit=%d output=%s", processExitCode(err), out)
	}
}

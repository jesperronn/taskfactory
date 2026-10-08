package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStubRunCreatesFileInTempWorktree(t *testing.T) {
	installStub(t)
	t.Setenv("ANTHROPIC_API_KEY", "must-not-leak")
	req := testRequest(t)
	res := Run(context.Background(), req, testOpts(okDial, testModel, testHaiku))
	if res.State != StateExit {
		t.Fatalf("state = %s, want exit; note=%s output=%s", res.State, res.Note, res.Output)
	}
	if res.ExitCode == nil || *res.ExitCode != 0 {
		t.Fatalf("exit code = %v, want 0", res.ExitCode)
	}
	if !strings.Contains(res.Note, "ANTHROPIC_DEFAULT_HAIKU_MODEL="+testHaiku) {
		t.Errorf("note does not record the haiku mapping: %s", res.Note)
	}
	if !strings.Contains(res.Note, "no time flag") {
		t.Errorf("note does not say the wall-clock bound is adapter-enforced: %s", res.Note)
	}

	data, err := os.ReadFile(filepath.Join(req.Worktree, "created.txt"))
	if err != nil {
		t.Fatalf("stub did not create file in worktree: %v", err)
	}
	if strings.TrimSpace(string(data)) != "stub ran" {
		t.Errorf("created.txt = %q", data)
	}

	argv, err := os.ReadFile(filepath.Join(req.Worktree, "argv.txt"))
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if !strings.Contains(string(argv), testModel) || strings.Contains(string(argv), testPrompt) {
		t.Errorf("argv must name the explicit model and omit the prompt: %s", argv)
	}

	stdin, err := os.ReadFile(filepath.Join(req.Worktree, "stdin.txt"))
	if err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	if strings.TrimSpace(string(stdin)) != testPrompt {
		t.Errorf("stdin = %q, want the prompt", stdin)
	}

	env, err := os.ReadFile(filepath.Join(req.Worktree, "env.txt"))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	want := strings.Join([]string{
		testBase,
		testToken,
		testModel,
		testModel,
		testHaiku,
		"[]",
		"1",
	}, "\n") + "\n"
	if string(env) != want {
		t.Errorf("child environment =\n%s\nwant\n%s", env, want)
	}
}

package omp

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const testModel = "omlx/Ornith-1.5-35B-A3B-MLX-4bit"

func TestArgvSelectsExplicitModel(t *testing.T) {
	req := Request{
		TaskID:   "TF-024",
		Worktree: "/abs/worktrees/TF-024",
		Model:    testModel,
		Timeout:  10 * time.Minute,
		Prompt:   "do the task",
	}
	argv, err := NewArgvBuilder().Argv(req)
	if err != nil {
		t.Fatalf("Argv: %v", err)
	}
	want := []string{
		"--model", testModel,
		"--cwd", "/abs/worktrees/TF-024",
		"--max-time", "600",
		"--no-session",
		"-p", "do the task",
	}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	joined := strings.Join(argv, " ")
	for _, forbidden := range []string{"--smol", "--slow", "--plan", "--auto-approve", "--fallback-model"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("argv contains forbidden flag %s: %q", forbidden, argv)
		}
	}

	req.Model = ""
	if _, err := NewArgvBuilder().Argv(req); err == nil {
		t.Error("Argv with empty model succeeded; want refusal (no default or fallback)")
	}
	req.Model = testModel
	req.Worktree = "relative/path"
	if _, err := NewArgvBuilder().Argv(req); err == nil {
		t.Error("Argv with relative worktree succeeded; want error")
	}
}

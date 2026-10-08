package pi

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const testModel = "Ornith-1.5-35B-A3B-MLX-4bit"

func TestArgvSelectsExplicitModel(t *testing.T) {
	req := Request{
		TaskID:   "TF-025",
		Worktree: "/abs/worktrees/TF-025",
		Model:    testModel,
		Timeout:  10 * time.Minute,
		Prompt:   "do the task",
	}
	argv, err := NewArgvBuilder().Argv(req)
	if err != nil {
		t.Fatalf("Argv: %v", err)
	}
	want := []string{
		"--provider", "omlx",
		"--model", testModel,
		"--thinking", "low",
		"--no-session",
		"-p", "do the task",
	}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	joined := strings.Join(argv, " ")
	for _, forbidden := range []string{"-a", "--approve", "--cwd", "--timeout", "--mode", "--models", "--fallback"} {
		for _, f := range argv {
			if f == forbidden {
				t.Errorf("argv contains forbidden flag %s: %q", forbidden, joined)
			}
		}
	}

	req.Model = ""
	if _, err := NewArgvBuilder().Argv(req); err == nil {
		t.Error("Argv with empty model succeeded; want refusal (no default or fallback)")
	}
	req.Model = "omlx/" + testModel
	if _, err := NewArgvBuilder().Argv(req); err == nil {
		t.Error("Argv with provider-prefixed model succeeded; want refusal (bare id only)")
	}
	req.Model = testModel
	req.Worktree = "relative/path"
	if _, err := NewArgvBuilder().Argv(req); err == nil {
		t.Error("Argv with relative worktree succeeded; want error")
	}
}

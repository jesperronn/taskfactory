package claude

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	testModel  = "omlx/Ornith-1.5-35B-A3B-MLX-4bit"
	testHaiku  = "omlx/Ornith-1.5-9B-MLX-4bit"
	testBase   = "http://127.0.0.1:8000"
	testToken  = "test-placeholder-token"
	testPrompt = "stub prompt"
)

func TestArgvUsesStdinAndNoFallback(t *testing.T) {
	req := Request{
		TaskID:     "TF-026",
		Worktree:   "/abs/worktrees/TF-026",
		Model:      testModel,
		HaikuModel: testHaiku,
		Timeout:    10 * time.Minute,
		Prompt:     testPrompt,
	}
	argv, err := NewArgvBuilder().Argv(req)
	if err != nil {
		t.Fatalf("Argv: %v", err)
	}
	want := []string{
		"-p",
		"--model", testModel,
		"--disallowedTools", "LSP",
		"--permission-mode", "acceptEdits",
		"--allowedTools", strings.Join(AllowedTools, ","),
	}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	joined := strings.Join(argv, " ")
	if strings.Contains(joined, testPrompt) {
		t.Errorf("argv carries the prompt; it must go on stdin: %q", argv)
	}
	for _, forbidden := range []string{"--fallback-model", "--smol", "--slow", "--plan", "--add-dir", "--max-budget-usd"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("argv contains forbidden flag %s: %q", forbidden, argv)
		}
	}

	base, err := BaseURL(DefaultEndpoint)
	if err != nil {
		t.Fatalf("BaseURL: %v", err)
	}
	if base != testBase {
		t.Fatalf("base URL = %q, want %q", base, testBase)
	}
	env := Environ([]string{"PATH=/bin", "ANTHROPIC_API_KEY=leak", "ANTHROPIC_BASE_URL=https://api.example.com", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=0"}, base, testToken, req)
	wantEnv := []string{
		"PATH=/bin",
		"ANTHROPIC_BASE_URL=" + testBase,
		"ANTHROPIC_AUTH_TOKEN=" + testToken,
		"ANTHROPIC_DEFAULT_OPUS_MODEL=" + testModel,
		"ANTHROPIC_DEFAULT_SONNET_MODEL=" + testModel,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL=" + testHaiku,
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
	}
	if !reflect.DeepEqual(env, wantEnv) {
		t.Fatalf("env = %q, want %q", env, wantEnv)
	}

	for _, bad := range []string{"api.example.com:443", "10.0.0.5:8000", "localhost:8000", "[2001:db8::1]:8000"} {
		if _, err := BaseURL(bad); err == nil {
			t.Errorf("BaseURL(%q) accepted a non-loopback endpoint", bad)
		}
	}
	if _, err := BaseURL("[::1]:8000"); err != nil {
		t.Errorf("BaseURL(::1) refused a loopback address: %v", err)
	}

	for name, mutate := range map[string]func(*Request){
		"empty model":       func(r *Request) { r.Model = "" },
		"empty haiku model": func(r *Request) { r.HaikuModel = "" },
		"relative worktree": func(r *Request) { r.Worktree = "relative/path" },
		"empty prompt":      func(r *Request) { r.Prompt = "" },
	} {
		bad := req
		mutate(&bad)
		if _, err := NewArgvBuilder().Argv(bad); err == nil {
			t.Errorf("Argv with %s succeeded; want refusal", name)
		}
	}
}

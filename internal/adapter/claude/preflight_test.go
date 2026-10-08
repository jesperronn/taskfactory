package claude

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// installStub writes a fake claude into a temp bin directory and makes that
// directory the only entry on PATH. The stub records its argv, stdin, and the
// ANTHROPIC_* values it received, then creates created.txt in its working
// directory. It never starts a model. The returned path is a launch marker in
// the stub directory that exists only if the stub ran.
func installStub(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	launchMarker := filepath.Join(binDir, "launched")
	script := `#!/bin/sh
printf '%s\n' "$@" > argv.txt
while IFS= read -r line || [ -n "$line" ]; do printf '%s\n' "$line"; done > stdin.txt
printf '%s\n' "$ANTHROPIC_BASE_URL" "$ANTHROPIC_AUTH_TOKEN" "$ANTHROPIC_DEFAULT_OPUS_MODEL" "$ANTHROPIC_DEFAULT_SONNET_MODEL" "$ANTHROPIC_DEFAULT_HAIKU_MODEL" "[$ANTHROPIC_API_KEY]" "$CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC" > env.txt
: > '` + launchMarker + `'
printf 'stub ran\n' > created.txt
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, Binary), []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("PATH", binDir)
	return launchMarker
}

func testRequest(t *testing.T) Request {
	t.Helper()
	return Request{
		TaskID:     "TF-026",
		Worktree:   t.TempDir(),
		Model:      testModel,
		HaikuModel: testHaiku,
		Timeout:    time.Minute,
		Prompt:     testPrompt,
	}
}

// testOpts injects a listing that reports listed and a dial that succeeds, so
// no socket opens and no real endpoint is queried.
func testOpts(dial func(context.Context, string) error, listed ...string) Options {
	return Options{
		AuthToken:  testToken,
		Dial:       dial,
		ListModels: func(context.Context, string, string) ([]string, error) { return listed, nil },
	}
}

func okDial(context.Context, string) error { return nil }

func TestPreflightRefusesBeforeLaunch(t *testing.T) {
	assertBlocked := func(t *testing.T, req Request, opts Options, marker string) {
		t.Helper()
		if pf := Preflight(context.Background(), req, opts); pf.Err() == nil {
			t.Fatal("preflight passed; want refusal")
		}
		if res := Run(context.Background(), req, opts); res.State != StateBlocked {
			t.Errorf("Run state = %s, want blocked; note=%s", res.State, res.Note)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatal("stub was launched despite refusal")
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(marker), "argv.txt")); err == nil {
			t.Fatal("argv was recorded despite refusal")
		}
	}

	t.Run("adapter binary missing", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		req := testRequest(t)
		assertBlocked(t, req, testOpts(okDial, testModel, testHaiku), filepath.Join(req.Worktree, "created.txt"))
	})

	t.Run("model not registered", func(t *testing.T) {
		marker := installStub(t)
		req := testRequest(t)
		opts := testOpts(okDial, "omlx/Some-Other-Model", testHaiku)
		pf := Preflight(context.Background(), req, opts)
		if pf.Err() == nil || !strings.Contains(pf.Err().Error(), testModel) {
			t.Errorf("model refusal does not name the model: %v", pf.Err())
		}
		assertBlocked(t, req, opts, marker)
	})

	t.Run("haiku model not registered", func(t *testing.T) {
		marker := installStub(t)
		req := testRequest(t)
		opts := testOpts(okDial, testModel)
		if pf := Preflight(context.Background(), req, opts); pf.Err() == nil || !strings.Contains(pf.Err().Error(), testHaiku) {
			t.Errorf("haiku refusal does not name the haiku model: %v", pf.Err())
		}
		assertBlocked(t, req, opts, marker)
	})

	t.Run("endpoint unreachable", func(t *testing.T) {
		marker := installStub(t)
		req := testRequest(t)
		down := func(context.Context, string) error { return errors.New("connection refused") }
		assertBlocked(t, req, testOpts(down, testModel, testHaiku), marker)
	})

	t.Run("non-loopback endpoint never dialed or queried", func(t *testing.T) {
		marker := installStub(t)
		req := testRequest(t)
		opts := testOpts(func(context.Context, string) error {
			t.Fatal("dial called for a non-loopback endpoint")
			return nil
		}, testModel, testHaiku)
		opts.Endpoint = "10.0.0.5:8000"
		opts.ListModels = func(context.Context, string, string) ([]string, error) {
			t.Fatal("model listing called for a non-loopback endpoint")
			return nil, nil
		}
		assertBlocked(t, req, opts, marker)
	})

	t.Run("auth token missing", func(t *testing.T) {
		marker := installStub(t)
		t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
		req := testRequest(t)
		opts := testOpts(okDial, testModel, testHaiku)
		opts.AuthToken = ""
		assertBlocked(t, req, opts, marker)
	})

	t.Run("all available", func(t *testing.T) {
		installStub(t)
		pf := Preflight(context.Background(), testRequest(t), testOpts(okDial, testModel, testHaiku))
		if err := pf.Err(); err != nil {
			t.Fatalf("preflight refused a fully available setup: %v", err)
		}
		if len(pf.Checks) != 6 {
			t.Errorf("checks = %d, want 6", len(pf.Checks))
		}
	})
}

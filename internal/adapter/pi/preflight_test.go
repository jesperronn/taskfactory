package pi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// installStub writes a fake pi into a temp bin directory and makes it the only
// entry on PATH. `pi --list-models` prints a listing that names listedModel on
// the omlx provider. Any other invocation records its argv and working
// directory, creates a launch marker, and writes created.txt in its working
// directory, so no real harness or model is started.
func installStub(t *testing.T, listedModel string) string {
	t.Helper()
	binDir := t.TempDir()
	launchMarker := filepath.Join(binDir, "launched")
	script := `#!/bin/sh
if [ "$1" = "--list-models" ]; then
  printf 'provider  model\nomlx      ` + listedModel + `\n'
  exit 0
fi
printf '%s\n' "$@" > argv.txt
pwd > cwd.txt
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
		TaskID:   "TF-025",
		Worktree: t.TempDir(),
		Model:    testModel,
		Timeout:  time.Minute,
		Prompt:   "stub prompt",
	}
}

func okDial(context.Context, string) error { return nil }

// testOpts isolates the run from the real home directory and the network.
func testOpts(t *testing.T, dial func(context.Context, string) error) Options {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	return Options{Dial: dial}
}

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
	}

	t.Run("adapter binary missing", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		req := testRequest(t)
		assertBlocked(t, req, testOpts(t, okDial), filepath.Join(req.Worktree, "created.txt"))
	})

	t.Run("model not listed", func(t *testing.T) {
		marker := installStub(t, "Some-Other-Model")
		req := testRequest(t)
		pf := Preflight(context.Background(), req, testOpts(t, okDial))
		if pf.Err() == nil || !strings.Contains(pf.Err().Error(), testModel) {
			t.Errorf("model refusal does not name the model: %v", pf.Err())
		}
		assertBlocked(t, req, testOpts(t, okDial), marker)
	})

	t.Run("endpoint unreachable", func(t *testing.T) {
		marker := installStub(t, testModel)
		req := testRequest(t)
		down := func(context.Context, string) error { return errors.New("connection refused") }
		assertBlocked(t, req, testOpts(t, down), marker)
	})

	t.Run("all available", func(t *testing.T) {
		installStub(t, testModel)
		pf := Preflight(context.Background(), testRequest(t), testOpts(t, okDial))
		if err := pf.Err(); err != nil {
			t.Fatalf("preflight refused a fully available setup: %v", err)
		}
		if len(pf.Checks) != 3 {
			t.Errorf("checks = %d, want 3", len(pf.Checks))
		}
	})
}

package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRunStallsAtAdapterTimeout runs a stub that sleeps past the bound. The
// run must be recorded as stalled, not as an exit.
func TestRunStallsAtAdapterTimeout(t *testing.T) {
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, Binary), []byte("#!/bin/sh\n/bin/sleep 5\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("PATH", binDir)
	req := testRequest(t)
	req.Timeout = time.Second
	res := Run(context.Background(), req, testOpts(okDial, testModel, testHaiku))
	if res.State != StateStalled {
		t.Fatalf("state = %s, want stalled; note=%s", res.State, res.Note)
	}
	if res.ExitCode != nil {
		t.Errorf("exit code = %d for a stalled run, want none", *res.ExitCode)
	}
}

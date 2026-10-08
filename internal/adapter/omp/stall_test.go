package omp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRunStallsAtAdapterTimeout runs a stub whose non-catalog invocation sleeps
// past the bound. The run must be recorded as stalled, not as an exit.
func TestRunStallsAtAdapterTimeout(t *testing.T) {
	binDir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = \"models\" ]; then printf '%s\\n' '{\"models\":[{\"selector\":\"" + testModel + "\"}]}'; exit 0; fi\n/bin/sleep 5\n"
	if err := os.WriteFile(filepath.Join(binDir, Binary), []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("PATH", binDir)
	req := testRequest(t)
	req.Timeout = time.Second
	res := Run(context.Background(), req, testOpts(t, okDial))
	if res.State != StateStalled {
		t.Fatalf("state = %s, want stalled; note=%s", res.State, res.Note)
	}
	if res.ExitCode != nil {
		t.Errorf("exit code = %d for a stalled run, want none", *res.ExitCode)
	}
}

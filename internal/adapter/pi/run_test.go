package pi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStubRunCreatesFileInTempWorktree(t *testing.T) {
	installStub(t, testModel)
	req := testRequest(t)
	res := Run(context.Background(), req, testOpts(t, okDial))
	if res.State != StateExit {
		t.Fatalf("state = %s, want exit; note=%s output=%s", res.State, res.Note, res.Output)
	}
	if res.ExitCode == nil || *res.ExitCode != 0 {
		t.Fatalf("exit code = %v, want 0", res.ExitCode)
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
	if !strings.Contains(string(argv), testModel) {
		t.Errorf("launched argv does not name the explicit model: %s", argv)
	}
	cwd, err := os.ReadFile(filepath.Join(req.Worktree, "cwd.txt"))
	if err != nil {
		t.Fatalf("read cwd: %v", err)
	}
	if strings.TrimSpace(string(cwd)) == "" {
		t.Error("stub recorded an empty working directory")
	}
}

func TestRunStallsAtAdapterTimeout(t *testing.T) {
	binDir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = \"--list-models\" ]; then printf 'omlx %s\\n' '" + testModel + "'; exit 0; fi\n/bin/sleep 5\n"
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
}

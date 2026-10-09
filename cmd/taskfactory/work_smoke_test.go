package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// smokeEndpointDefault is the project-local oMLX server the smoke test needs.
// TASKFACTORY_SMOKE_ENDPOINT overrides it, which lets the dial-skip path be
// shown against a closed port.
const smokeEndpointDefault = "127.0.0.1:8000"

// smokeTaskID is the one-line task claimed in the scratch repository.
const smokeTaskID = "TF-901"

// smokeTask is the one-line task the worker is asked to complete.
const smokeTask = "# TF-901: Smoke\n\n## Goal\n\nCreate smoke.txt with one line.\n\n## Dependencies\n\nNone\n\n## Scope\n\nsmoke.txt only.\n\n## Constraints\n\nKeep it to one line.\n\n## Success criteria\n\n### C1: smoke.txt has content\n\nCheck: test -s smoke.txt\n\n## Verification\n\nRun check.\n"

// TestWorkSmokeAgainstLocalOMLX is opt-in. It runs only when TASKFACTORY_SMOKE=1
// is set and the endpoint (127.0.0.1:8000 unless TASKFACTORY_SMOKE_ENDPOINT
// overrides it) accepts a TCP connection; otherwise it skips. When
// enabled it needs TASKFACTORY_SMOKE_ADAPTER and TASKFACTORY_SMOKE_MODEL, with
// no defaults, and asserts only that work returned a result state and wrote a
// log, not that the model succeeded.
func TestWorkSmokeAgainstLocalOMLX(t *testing.T) {
	if os.Getenv("TASKFACTORY_SMOKE") != "1" {
		t.Skip("smoke test skipped: set TASKFACTORY_SMOKE=1 to run it")
	}
	endpoint := os.Getenv("TASKFACTORY_SMOKE_ENDPOINT")
	if endpoint == "" {
		endpoint = smokeEndpointDefault
	}
	conn, err := net.DialTimeout("tcp", endpoint, 2*time.Second)
	if err != nil {
		t.Skipf("smoke test skipped: nothing accepts TCP connections at %s: %v", endpoint, err)
	}
	_ = conn.Close()

	adapter := os.Getenv("TASKFACTORY_SMOKE_ADAPTER")
	model := os.Getenv("TASKFACTORY_SMOKE_MODEL")
	if adapter == "" || model == "" {
		t.Fatal("TASKFACTORY_SMOKE_ADAPTER and TASKFACTORY_SMOKE_MODEL must both be set; there is no default")
	}

	binary := buildCLI(t)
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		full := append([]string{"-C", root}, args...)
		if output, err := exec.Command("git", full...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
		}
	}
	git("init", "--quiet", "--initial-branch=main")
	// Local config only: the scratch repo never signs and needs an identity.
	git("config", "commit.gpgsign", "false")
	git("config", "user.name", "Smoke")
	git("config", "user.email", "smoke@example.invalid")
	if output, err := runCLI(t, binary, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, output)
	}
	instructions, err := os.ReadFile(filepath.Join("..", "..", "docs", "worker-instructions.md"))
	if err != nil {
		t.Fatalf("read worker instructions: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "worker-instructions.md"), instructions, 0o600); err != nil {
		t.Fatal(err)
	}
	taskPath := filepath.Join(root, "tasks", "ready", smokeTaskID+"-smoke.md")
	if err := os.WriteFile(taskPath, []byte(smokeTask), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "--quiet", "-m", "fixture")
	if output, err := runCLI(t, binary, root, "claim", smokeTaskID, "--owner", "smoke"); err != nil {
		t.Fatalf("claim: %v\n%s", err, output)
	}

	output, err := runCLI(t, binary, root, "work", smokeTaskID, "--adapter", adapter, "--model", model, "--timeout", "8m")
	code := processExitCode(err)
	if code != 0 && code != 1 {
		t.Fatalf("work exit code = %d, want a result state (0 or 1); output:\n%s", code, output)
	}
	logs, err := filepath.Glob(filepath.Join(root, ".taskfactory", "logs", smokeTaskID, "*.log"))
	if err != nil || len(logs) == 0 {
		t.Fatalf("work wrote no log under .taskfactory/logs/%s; output:\n%s", smokeTaskID, output)
	}
	t.Logf("work exit code %d, log %s\n%s", code, logs[0], output)
}

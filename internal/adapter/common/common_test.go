package common

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// installBinary writes a shell script named binary into a temp directory and
// makes that directory the only entry on PATH. No real harness is started.
func installBinary(t *testing.T, binary, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, binary), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("PATH", dir)
}

func TestPreflightResultErrJoinsFailures(t *testing.T) {
	ok := PreflightResult{Checks: []PreflightCheck{{Name: "a", Err: nil}}}
	if err := ok.Err(); err != nil {
		t.Fatalf("Err() = %v for all-passing checks, want nil", err)
	}
	bad := PreflightResult{Checks: []PreflightCheck{
		{Name: "first", Err: errors.New("one")},
		{Name: "second", Err: nil},
		{Name: "third", Err: errors.New("three")},
	}}
	msg := bad.Err().Error()
	for _, want := range []string{"first: one", "third: three"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Err() = %q, missing %q", msg, want)
		}
	}
	if strings.Contains(msg, "second") {
		t.Errorf("Err() = %q names a passing check", msg)
	}
}

func TestEndpointDefaultsWhenEmpty(t *testing.T) {
	if got := Endpoint("", "127.0.0.1:8000"); got != "127.0.0.1:8000" {
		t.Errorf("Endpoint(empty) = %q, want the default", got)
	}
	if got := Endpoint("127.0.0.1:9000", "127.0.0.1:8000"); got != "127.0.0.1:9000" {
		t.Errorf("Endpoint(configured) = %q, want the configured value", got)
	}
}

func TestEndpointCheckUsesInjectedDial(t *testing.T) {
	var seen string
	ok := EndpointCheck(context.Background(), "127.0.0.1:8000", func(_ context.Context, addr string) error {
		seen = addr
		return nil
	})
	if ok.Err != nil || ok.Name != "endpoint 127.0.0.1:8000" || seen != "127.0.0.1:8000" {
		t.Errorf("check = %+v, seen %q; want named passing check dialed once", ok, seen)
	}
	down := EndpointCheck(context.Background(), "127.0.0.1:8000", func(context.Context, string) error {
		return errors.New("connection refused")
	})
	if down.Err == nil {
		t.Error("unreachable endpoint check passed; want failure")
	}
}

func TestCheckDirRefusesNonDirectory(t *testing.T) {
	if err := CheckDir(t.TempDir()); err != nil {
		t.Errorf("CheckDir(existing dir) = %v", err)
	}
	missing := filepath.Join(t.TempDir(), "absent")
	if err := CheckDir(missing); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("CheckDir(missing) = %v, want not-a-directory refusal", err)
	}
}

func TestLaunchMapsExitStallAndBlocked(t *testing.T) {
	t.Run("exit code is kept", func(t *testing.T) {
		installBinary(t, "tool", "exit 3\n")
		res := Launch(context.Background(), LaunchSpec{
			Binary: "tool", Dir: t.TempDir(), Timeout: time.Minute, Notes: "note",
		})
		if res.State != StateExit || res.ExitCode == nil || *res.ExitCode != 3 {
			t.Fatalf("result = %+v, want exit with code 3", res)
		}
		if res.Note != "note" {
			t.Errorf("note = %q, want the spec notes", res.Note)
		}
	})

	t.Run("timeout is stalled", func(t *testing.T) {
		installBinary(t, "tool", "/bin/sleep 5\n")
		res := Launch(context.Background(), LaunchSpec{
			Binary: "tool", Dir: t.TempDir(), Timeout: time.Second, Notes: "note",
		})
		if res.State != StateStalled || res.ExitCode != nil {
			t.Fatalf("result = %+v, want stalled with no exit code", res)
		}
		if !strings.HasPrefix(res.Note, "timeout exceeded; ") {
			t.Errorf("note = %q, want the timeout prefix", res.Note)
		}
	})

	t.Run("missing binary is blocked", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		res := Launch(context.Background(), LaunchSpec{
			Binary: "tool", Dir: t.TempDir(), Timeout: time.Minute,
		})
		if res.State != StateBlocked || res.ExitCode != nil {
			t.Fatalf("result = %+v, want blocked with no exit code", res)
		}
	})
}

package claim

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWithClaimLockSerializesProcesses(t *testing.T) {
	root := t.TempDir()
	ready := filepath.Join(root, "ready")
	acquired := filepath.Join(root, "acquired")
	locked := make(chan struct{})
	release := make(chan struct{})
	parentDone := make(chan error, 1)
	go func() {
		parentDone <- WithClaimLock(root, func() error {
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestClaimLockHelperProcess$")
	cmd.Env = append(os.Environ(), "TASKFACTORY_LOCK_HELPER=1", "TASKFACTORY_LOCK_ROOT="+root,
		"TASKFACTORY_LOCK_READY="+ready, "TASKFACTORY_LOCK_MARKER="+acquired)
	childDone := make(chan error, 1)
	go func() { childDone <- cmd.Run() }()

	if err := waitForFile(ready, childDone, 5*time.Second); err != nil {
		close(release)
		t.Fatalf("subprocess did not reach lock attempt: %v", err)
	}
	if err := assertFileRemainsAbsent(acquired, childDone, 200*time.Millisecond); err != nil {
		close(release)
		t.Fatalf("subprocess was not blocked by claim lock: %v", err)
	}
	close(release)
	if err := <-parentDone; err != nil {
		t.Fatal(err)
	}
	if err := <-childDone; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(acquired); err != nil {
		t.Fatalf("subprocess did not acquire lock after release: %v", err)
	}
}

func waitForFile(path string, childDone <-chan error, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect handshake file: %w", err)
		}
		select {
		case err := <-childDone:
			return fmt.Errorf("subprocess exited before handshake: %v", err)
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for %s", path)
		case <-ticker.C:
		}
	}
}

func assertFileRemainsAbsent(path string, childDone <-chan error, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("acquisition marker appeared before lock release")
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect acquisition marker: %w", err)
		}
		select {
		case err := <-childDone:
			return fmt.Errorf("subprocess exited before lock release: %v", err)
		case <-deadline.C:
			return nil
		case <-ticker.C:
		}
	}
}

func TestClaimLockHelperProcess(t *testing.T) {
	if os.Getenv("TASKFACTORY_LOCK_HELPER") != "1" {
		return
	}
	root := os.Getenv("TASKFACTORY_LOCK_ROOT")
	ready := os.Getenv("TASKFACTORY_LOCK_READY")
	marker := os.Getenv("TASKFACTORY_LOCK_MARKER")
	if err := os.WriteFile(ready, []byte("attempting"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WithClaimLock(root, func() error { return os.WriteFile(marker, []byte("acquired"), 0o600) }); err != nil {
		t.Fatal(err)
	}
}

func TestWithClaimLockReleasesAfterCallbackError(t *testing.T) {
	root := t.TempDir()
	want := fmt.Errorf("callback failed")
	if err := WithClaimLock(root, func() error { return want }); err != want {
		t.Fatalf("WithClaimLock error = %v, want callback error", err)
	}
	called := false
	if err := WithClaimLock(root, func() error { called = true; return nil }); err != nil {
		t.Fatalf("second lock call: %v", err)
	}
	if !called {
		t.Fatal("callback was not called after prior callback error")
	}
}

func TestEligibility(t *testing.T) {
	tests := []struct {
		name       string
		active     int
		dependency string
		archiveDep bool
		wantErr    string
	}{
		{name: "capacity available", active: 3},
		{name: "capacity full", active: 4, wantErr: "workers.max_parallel"},
		{name: "below capacity", active: 2},
		{name: "missing dependency", dependency: "TF-002", wantErr: "TF-002"},
		{name: "unarchived dependency", dependency: "TF-002", wantErr: "TF-002 is unresolved"},
		{name: "archived dependency", dependency: "TF-002", archiveDep: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newProject(t)
			if tt.dependency != "" {
				appendDependency(t, root, tt.dependency)
				if tt.archiveDep {
					writeTask(t, root, "archive", "TF-002-prerequisite.md", taskText("TF-002", "Prerequisite", "None"))
				} else if tt.wantErr == "TF-002 is unresolved" {
					writeTask(t, root, "ready", "TF-002-prerequisite.md", taskText("TF-002", "Prerequisite", "None"))
				}
			}
			for i := 0; i < tt.active; i++ {
				writeTask(t, root, "active", fmt.Sprintf("TF-%03d-worker.md", i+20), "# active placeholder\n")
			}
			before := snapshotTree(t, root)
			err := CheckEligibility(root, "TF-001")
			after := snapshotTree(t, root)
			if !equalSnapshot(before, after) {
				t.Fatal("eligibility check changed task paths or bytes")
			}
			if tt.wantErr == "" && err != nil {
				t.Fatalf("CheckEligibility: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("CheckEligibility error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestEligibilityRejectsUnknownAndInvalidReadyTasksReadOnly(t *testing.T) {
	root := newProject(t)
	before := snapshotTree(t, root)
	if err := CheckEligibility(root, "TF-999"); err == nil || !strings.Contains(err.Error(), "TF-999") {
		t.Fatalf("unknown task error = %v", err)
	}
	if after := snapshotTree(t, root); !equalSnapshot(before, after) {
		t.Fatal("unknown-task eligibility check changed task paths or bytes")
	}
	writeTask(t, root, "ready", "TF-003-invalid.md", "# TF-003: Invalid\n")
	before = snapshotTree(t, root)
	if err := CheckEligibility(root, "TF-003"); err == nil || !strings.Contains(err.Error(), "TF-003") {
		t.Fatalf("invalid task error = %v", err)
	}
	if after := snapshotTree(t, root); !equalSnapshot(before, after) {
		t.Fatal("eligibility check changed task paths or bytes")
	}
}

func newProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		if err := os.MkdirAll(filepath.Join(root, "tasks", state), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, ".taskfactory"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "protocol_version = 1\n[workers]\nmax_parallel = 4\n"
	if err := os.WriteFile(filepath.Join(root, ".taskfactory", "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	writeTask(t, root, "ready", "TF-001-target.md", taskText("TF-001", "Target", "None"))
	return root
}

func taskText(id, title, deps string) string {
	return fmt.Sprintf("# %s: %s\n\n## Goal\n\nDo it.\n\n## Dependencies\n\n%s\n\n## Scope\n\nImplement it.\n\n## Constraints\n\nKeep it small.\n\n## Success criteria\n\n### C1: It works\n\nCheck: true\n\n## Verification\n\nRun the check.\n", id, title, deps)
}

func appendDependency(t *testing.T, root, dep string) {
	t.Helper()
	path := filepath.Join(root, "tasks", "ready", "TF-001-target.md")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contents = []byte(strings.Replace(string(contents), "## Dependencies\n\nNone", "## Dependencies\n\n- "+dep, 1))
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeTask(t *testing.T, root, state, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "tasks", state, name), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

type fileSnapshot map[string]string

func snapshotTree(t *testing.T, root string) fileSnapshot {
	t.Helper()
	result := fileSnapshot{}
	err := filepath.WalkDir(filepath.Join(root, "tasks"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func equalSnapshot(a, b fileSnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for path, contents := range a {
		if b[path] != contents {
			return false
		}
	}
	return true
}

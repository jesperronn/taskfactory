package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeStateConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".taskfactory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".taskfactory", "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTasksDirConfigAcceptsAndCleans(t *testing.T) {
	for value, want := range map[string]string{
		".taskfactory/tasks":   ".taskfactory/tasks",
		"./a//b/":              "a/b",
		"tasks":                "tasks",
		"work/items":           "work/items",
		".taskfactory/tasks/x": ".taskfactory/tasks/x",
	} {
		root := t.TempDir()
		writeStateConfig(t, root, "tasks_dir = \""+value+"\"\n")
		if _, err := Load(root); err != nil {
			t.Errorf("Load with tasks_dir %q: %v", value, err)
		}
		got, err := TasksDirOf(root)
		if err != nil || got != want {
			t.Errorf("TasksDirOf(%q) = %q, %v; want %q", value, got, err, want)
		}
	}
}

func TestTasksDirConfigRejects(t *testing.T) {
	for _, value := range []string{
		"", "/abs/tasks", "../tasks", "a/../b", "a/..", ".", "./", "a\\\\b", "..",
		".taskfactory", ".taskfactory/", ".taskfactory/evidence", ".taskfactory/logs",
		".taskfactory/worktrees", ".taskfactory/worktrees/x", ".taskfactory/integration-evidence",
		".taskfactory/integration-evidence/y", ".taskfactory/config.toml",
	} {
		root := t.TempDir()
		writeStateConfig(t, root, "tasks_dir = \""+value+"\"\n")
		want := "tasks_dir must be a relative path inside the state directory"
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "config.toml") {
			t.Errorf("Load with tasks_dir %q: err = %v", value, err)
		}
		if _, err := TasksDirOf(root); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("TasksDirOf(%q) err = %v", value, err)
		}
	}
}

func TestTasksDirConfigRejectsSymlinkOutOfState(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	writeStateConfig(t, root, "tasks_dir = \"link/tasks\"\n")
	if _, err := TasksDirOf(root); err == nil || !strings.Contains(err.Error(), "tasks_dir must be a relative path inside the state directory") {
		t.Fatalf("err = %v", err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("Load accepted a symlink leaving the state root")
	}
	// A symlink that stays inside the state root is fine.
	if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "inner")); err != nil {
		t.Fatal(err)
	}
	writeStateConfig(t, root, "tasks_dir = \"inner/tasks\"\n")
	if _, err := TasksDirOf(root); err != nil {
		t.Fatalf("inside symlink rejected: %v", err)
	}
}

func TestTasksDirOfDefaultsAndReadsOnlyTheKey(t *testing.T) {
	root := t.TempDir()
	if got, err := TasksDirOf(root); err != nil || got != "tasks" {
		t.Fatalf("missing file: %q, %v", got, err)
	}
	writeStateConfig(t, root, "protocol_version = 1\n[workers]\nmax_parallel = 99\nunknown = true\n")
	if got, err := TasksDirOf(root); err != nil || got != "tasks" {
		t.Fatalf("missing key: %q, %v", got, err)
	}
	writeStateConfig(t, root, "tasks_dir = \".taskfactory/tasks\"\n[workers]\nmax_parallel = 99\nunknown = true\n")
	if got, err := TasksDirOf(root); err != nil || got != ".taskfactory/tasks" {
		t.Fatalf("with key and otherwise invalid config: %q, %v", got, err)
	}
	writeStateConfig(t, root, "tasks_dir = [\n")
	if _, err := TasksDirOf(root); err == nil {
		t.Fatal("malformed TOML accepted")
	}
}

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/config"
)

func TestCLIHelpVersionAndInvalidFlag(t *testing.T) {
	binary := buildCLI(t)
	outsideCheckout := t.TempDir()

	tests := []struct {
		name       string
		args       []string
		exitCode   int
		wantOutput []string
		wantAbsent []string
	}{
		{name: "help", args: []string{"--help"}, exitCode: 0, wantOutput: []string{"taskfactory", "--help", "--version"}},
		{name: "version", args: []string{"--version"}, exitCode: 0, wantOutput: []string{"taskfactory version dev"}},
		{name: "invalid flag", args: []string{"--not-a-global-flag"}, exitCode: 2, wantOutput: []string{"flag provided but not defined", "not-a-global-flag"}, wantAbsent: []string{"Usage:"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binary, tt.args...)
			cmd.Dir = outsideCheckout
			output, err := cmd.CombinedOutput()
			if got := processExitCode(err); got != tt.exitCode {
				t.Fatalf("exit code = %d, want %d; output: %s", got, tt.exitCode, output)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(string(output), want) {
					t.Errorf("output %q does not contain %q", output, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(string(output), absent) {
					t.Errorf("output %q unexpectedly contains %q", output, absent)
				}
			}
		})
	}
}

func TestInitCreatesLoadableDefaultProject(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)

	output, err := runCLI(t, binary, root, "init")
	if err != nil {
		t.Fatalf("taskfactory init failed: %v\n%s", err, output)
	}

	loaded, err := config.Load(root)
	if err != nil {
		t.Fatalf("created config is not loadable: %v", err)
	}
	if loaded.ProtocolVersion != 1 || loaded.Workers.MaxParallel != 4 || !loaded.Git.UseWorktrees || loaded.Git.IntegrationStrategy != "ff-only" {
		t.Errorf("loaded defaults = %#v", loaded)
	}
	if loaded.Verification.Worker == nil || loaded.Verification.Integration == nil {
		t.Errorf("default verification commands were not loaded: %#v", loaded.Verification)
	}
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		path := filepath.Join(root, "tasks", state)
		info, statErr := os.Stat(path)
		if statErr != nil || !info.IsDir() {
			t.Errorf("state directory %s was not created: %v", path, statErr)
		}
	}
}

func TestInitIsIdempotentAndCreatesMissingStateDirectory(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	if output, err := runCLI(t, binary, root, "init"); err != nil {
		t.Fatalf("first init failed: %v\n%s", err, output)
	}

	configPath := filepath.Join(root, ".taskfactory", "config.toml")
	taskPath := filepath.Join(root, "tasks", "ready", "TF-999-example.md")
	if err := os.WriteFile(taskPath, []byte("user task content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configBefore, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configInfoBefore, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	taskInfoBefore, err := os.Stat(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "tasks", "failed")); err != nil {
		t.Fatal(err)
	}

	if output, err := runCLI(t, binary, root, "init"); err != nil {
		t.Fatalf("second init failed: %v\n%s", err, output)
	}
	configAfter, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configInfoAfter, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	taskAfter, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	taskInfoAfter, err := os.Stat(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(configBefore, configAfter) || !configInfoBefore.ModTime().Equal(configInfoAfter.ModTime()) {
		t.Errorf("config changed on rerun: bytes equal=%t, mtime before=%s after=%s", bytes.Equal(configBefore, configAfter), configInfoBefore.ModTime(), configInfoAfter.ModTime())
	}
	if string(taskAfter) != "user task content\n" || !taskInfoBefore.ModTime().Equal(taskInfoAfter.ModTime()) {
		t.Errorf("existing task changed on rerun: content=%q, mtime before=%s after=%s", taskAfter, taskInfoBefore.ModTime(), taskInfoAfter.ModTime())
	}
	if info, err := os.Stat(filepath.Join(root, "tasks", "failed")); err != nil || !info.IsDir() {
		t.Errorf("missing state directory was not restored: %v", err)
	}
}

func TestInitUsesGitTopLevelFromNestedDirectory(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	nested := filepath.Join(root, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if output, err := runCLI(t, binary, nested, "init"); err != nil {
		t.Fatalf("init from nested directory failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(root, ".taskfactory", "config.toml")); err != nil {
		t.Fatalf("config was not created at Git root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, ".taskfactory")); !os.IsNotExist(err) {
		t.Fatalf("nested directory unexpectedly received config: err=%v", err)
	}
}

func TestInitFailsOutsideGitProject(t *testing.T) {
	binary := buildCLI(t)
	outside := t.TempDir()
	output, err := runCLI(t, binary, outside, "init")
	if processExitCode(err) == 0 || !strings.Contains(string(output), "Git") {
		t.Fatalf("init outside Git should fail clearly; err=%v output=%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(outside, ".taskfactory")); !os.IsNotExist(err) {
		t.Fatalf("init outside Git changed directory: err=%v", err)
	}
}

func TestInitReportsExistingConflictingConfigWithoutOverwriting(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	configPath := filepath.Join(root, ".taskfactory", "config.toml")
	conflict := []byte("protocol_version = 99\n")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, conflict, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := runCLI(t, binary, root, "init")
	if processExitCode(err) == 0 || !strings.Contains(string(output), "protocol_version 99") {
		t.Fatalf("init should report conflicting config; err=%v output=%s", err, output)
	}
	after, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(after, conflict) {
		t.Fatalf("conflicting config was overwritten: %q", after)
	}
}

func TestInitDoesNotFollowProjectDirectorySymlinkOutsideGitRoot(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, ".taskfactory")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	output, err := runCLI(t, binary, root, "init")
	if processExitCode(err) == 0 || !strings.Contains(string(output), ".taskfactory") {
		t.Fatalf("init should reject a project directory symlink; err=%v output=%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(external, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("init wrote configuration outside Git root: err=%v", err)
	}
}

func buildCLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "taskfactory")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build taskfactory: %v\n%s", err, output)
	}
	return binary
}

func initGitProject(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project with trailing space ")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "--quiet", root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init temporary project: %v\n%s", err, output)
	}
	return root
}

func runCLI(t *testing.T, binary, directory string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = directory
	return cmd.CombinedOutput()
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

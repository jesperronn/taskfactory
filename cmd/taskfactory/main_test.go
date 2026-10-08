package main

import (
	"bytes"
	"crypto/sha256"
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
		{name: "help", args: []string{"--help"}, exitCode: 0, wantOutput: []string{"taskfactory", "--help", "--version", "init", "initialize project config", "integrate <ID>", "check-main", "recheck a stopped main"}},
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

func TestHelpListsAndDocumentsEveryTopLevelCommand(t *testing.T) {
	binary := buildCLI(t)
	outsideCheckout := t.TempDir()
	top, err := runCLI(t, binary, outsideCheckout, "--help")
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, top)
	}
	if len(commandHelps) != len(commandSummaries) {
		t.Errorf("commandHelps has %d entries, want %d", len(commandHelps), len(commandSummaries))
	}
	for _, command := range commandSummaries {
		if !strings.Contains(string(top), command.synopsis) || !strings.Contains(string(top), command.summary) {
			t.Errorf("top-level help does not list %q: %s", command.name, top)
		}
		help, ok := commandHelps[command.name]
		if !ok {
			t.Errorf("command %q has no per-command help", command.name)
			continue
		}
		if !strings.HasPrefix(help, "Usage: taskfactory "+command.synopsis+"\n") {
			t.Errorf("help for %q does not start with its usage line: %q", command.name, help)
		}
	}
}

func TestHelpPrintsCommandUsageToStdout(t *testing.T) {
	binary := buildCLI(t)
	outsideCheckout := t.TempDir()
	for _, command := range commandSummaries {
		for _, helpFlag := range []string{"--help", "-h"} {
			t.Run(command.name+" "+helpFlag, func(t *testing.T) {
				cmd := exec.Command(binary, command.name, helpFlag)
				cmd.Dir = outsideCheckout
				var stdout, stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				err := cmd.Run()
				if got := processExitCode(err); got != 0 {
					t.Fatalf("exit code = %d, want 0; stderr: %s", got, stderr.String())
				}
				if wantUsage := "Usage: taskfactory " + command.synopsis + "\n"; !strings.HasPrefix(stdout.String(), wantUsage) {
					t.Errorf("stdout %q does not start with usage line %q", stdout.String(), wantUsage)
				}
				if stdout.String() != commandHelps[command.name] {
					t.Errorf("stdout differs from the command help constant:\n%s", stdout.String())
				}
				if stderr.Len() != 0 {
					t.Errorf("stderr = %q, want empty", stderr.String())
				}
			})
		}
	}
}

func TestHelpColorFollowsFORCEAndNOCOLOR(t *testing.T) {
	binary := buildCLI(t)
	outsideCheckout := t.TempDir()
	run := func(extraEnv ...string) string {
		t.Helper()
		var env []string
		for _, kv := range os.Environ() {
			if strings.HasPrefix(kv, "NO_COLOR=") || strings.HasPrefix(kv, "FORCE_COLOR=") || strings.HasPrefix(kv, "TERM=") {
				continue
			}
			env = append(env, kv)
		}
		cmd := exec.Command(binary, "--help")
		cmd.Dir = outsideCheckout
		cmd.Env = append(env, extraEnv...)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("--help with %v failed: %v", extraEnv, err)
		}
		return string(output)
	}

	plain := run()
	if strings.Contains(plain, "\x1b[") {
		t.Errorf("piped output contains escapes: %q", plain)
	}
	if noColor := run("NO_COLOR=1", "FORCE_COLOR=1"); noColor != plain || strings.Contains(noColor, "\x1b[") {
		t.Errorf("NO_COLOR output must be plain and identical to piped output: %q", noColor)
	}
	forced := run("FORCE_COLOR=1")
	for _, command := range commandSummaries {
		if want := "\x1b[33m" + command.name + "\x1b[0m"; !strings.Contains(forced, want) {
			t.Errorf("FORCE_COLOR output lacks yellow %q", command.name)
		}
	}
	if stripped := stripSGR(forced); stripped != plain {
		t.Errorf("FORCE_COLOR output differs from plain text after stripping escapes")
	}

	for _, command := range commandSummaries {
		cmd := exec.Command(binary, command.name, "--help")
		cmd.Dir = outsideCheckout
		var env []string
		for _, kv := range os.Environ() {
			if !strings.HasPrefix(kv, "NO_COLOR=") && !strings.HasPrefix(kv, "FORCE_COLOR=") && !strings.HasPrefix(kv, "TERM=") {
				env = append(env, kv)
			}
		}
		cmd.Env = append(env, "FORCE_COLOR=1")
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s --help with FORCE_COLOR failed: %v", command.name, err)
		}
		if want := "\x1b[33m" + command.name + "\x1b[0m"; !strings.Contains(string(output), want) {
			t.Errorf("%s --help lacks yellow subcommand name", command.name)
		}
	}
}

// stripSGR removes ANSI SGR escape sequences of the form ESC [ ... m.
func stripSGR(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			if j := strings.IndexByte(s[i:], 'm'); j >= 0 {
				i += j
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestHelpWorksOutsideProjectAndChangesNothing(t *testing.T) {
	binary := buildCLI(t)
	outside := t.TempDir()
	project := initGitProject(t)
	if output, err := runCLI(t, binary, project, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, output)
	}
	taskPath := filepath.Join(project, "tasks", "ready", "TF-110-help.md")
	if err := os.WriteFile(taskPath, []byte(validTask("TF-110")), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(project, ".taskfactory", "config.toml")
	configBefore, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	before := taskTreeHashes(t, project)

	bare := initGitProject(t)
	cases := []struct {
		dir  string
		args []string
	}{
		{outside, []string{"init", "--help"}},
		{outside, []string{"status", "-h"}},
		{outside, []string{"validate", "--help"}},
		{outside, []string{"claim", "TF-999", "--help"}},
		{outside, []string{"verify", "--help", "extra"}},
		{outside, []string{"integrate", "-h"}},
		{outside, []string{"check-main", "--help"}},
		{project, []string{"claim", "TF-110", "--help"}},
		{project, []string{"claim", "TF-110", "--owner", "", "-h"}},
		{project, []string{"verify", "TF-110", "--help"}},
		{project, []string{"integrate", "TF-110", "--help"}},
		{project, []string{"check-main", "--help"}},
		{project, []string{"status", "--bogus", "--help"}},
		{bare, []string{"status", "--help"}},
		{bare, []string{"init", "--help"}},
	}
	for _, tc := range cases {
		output, err := runCLI(t, binary, tc.dir, tc.args...)
		if got := processExitCode(err); got != 0 || !strings.HasPrefix(string(output), "Usage: taskfactory "+tc.args[0]) {
			t.Errorf("%v in %s: exit=%d, output=%q", tc.args, tc.dir, got, output)
		}
	}

	if configAfter, err := os.ReadFile(configPath); err != nil || !bytes.Equal(configBefore, configAfter) {
		t.Errorf("config changed by help: err=%v", err)
	}
	if after := taskTreeHashes(t, project); !equalTaskHashes(before, after) {
		t.Error("help modified task files")
	}
	if _, err := os.Stat(filepath.Join(project, ".taskfactory", "evidence")); !os.IsNotExist(err) {
		t.Errorf("help created evidence directory: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, ".taskfactory")); !os.IsNotExist(err) {
		t.Errorf("help created project state outside a project: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(bare, ".taskfactory")); !os.IsNotExist(err) {
		t.Errorf("help created project state in an uninitialized project: err=%v", err)
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

func TestValidateSupportsWholeTreeAndSingleFile(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	if output, err := runCLI(t, binary, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, output)
	}
	taskPath := filepath.Join(root, "tasks", "ready", "TF-101-example.md")
	valid := "# TF-101: Example\n\n## Goal\n\nDo it.\n\n## Dependencies\n\nNone\n\n## Scope\n\nImplement.\n\n## Constraints\n\nKeep it small.\n\n## Success criteria\n\n### C1: It works\n\nCheck: go test ./...\n\n## Verification\n\nRun check.\n"
	if err := os.WriteFile(taskPath, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := runCLI(t, binary, root, "validate"); err != nil {
		t.Fatalf("whole tree: %v\n%s", err, output)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if output, err := runCLI(t, binary, nested, "validate", filepath.Join("tasks", "ready", "TF-101-example.md")); err != nil {
		t.Fatalf("relative path from nested directory: %v\n%s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "tasks", "ready", "TF-102-unrelated.md"), []byte("# TF-102: Broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := runCLI(t, binary, root, "validate", taskPath); err != nil {
		t.Fatalf("single file reported unrelated invalid file: %v\n%s", err, output)
	}
	if err := os.WriteFile(taskPath, []byte("# TF-101: Broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := runCLI(t, binary, root, "validate", taskPath); processExitCode(err) == 0 || !strings.Contains(string(output), "Goal") {
		t.Fatalf("invalid selected file should fail with field: %v\n%s", err, output)
	}
}

func TestClaimCLIRequiresOwnerAndActivatesTask(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	if output, err := runCLI(t, binary, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, output)
	}
	taskPath := filepath.Join(root, "tasks", "ready", "TF-108-claim.md")
	if err := os.WriteFile(taskPath, []byte(validTask("TF-108")), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", root, "add", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	cmd = exec.Command("git", "-C", root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "-m", "fixture")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, output)
	}
	if output, err := runCLI(t, binary, root, "claim", "TF-108"); processExitCode(err) == 0 || !strings.Contains(string(output), "--owner") {
		t.Fatalf("missing owner should fail: %v\n%s", err, output)
	}
	if output, err := runCLI(t, binary, root, "claim", "TF-108", "--owner", "worker"); err != nil || !strings.Contains(string(output), "claimed TF-108") {
		t.Fatalf("claim: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "active", "TF-108-claim.md")); err != nil {
		t.Fatalf("active task missing: %v", err)
	}
}

func TestVerifyCLIReturnsFailureAfterRecordingEvidence(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	if output, err := runCLI(t, binary, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, output)
	}
	task := strings.Replace(validTask("TF-109"), "Check: go test ./...", "Check: false", 1)
	taskPath := filepath.Join(root, "tasks", "ready", "TF-109-failing-check.md")
	if err := os.WriteFile(taskPath, []byte(task), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", root, "add", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}
	cmd = exec.Command("git", "-C", root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "-m", "fixture")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, output)
	}
	if output, err := runCLI(t, binary, root, "claim", "TF-109", "--owner", "worker"); err != nil {
		t.Fatalf("claim: %v\n%s", err, output)
	}
	output, err := runCLI(t, binary, root, "verify", "TF-109")
	if processExitCode(err) != 1 || !strings.Contains(string(output), "FAILED") {
		t.Fatalf("verify exit=%d err=%v output=%s", processExitCode(err), err, output)
	}
	evidence, err := os.ReadFile(filepath.Join(root, ".taskfactory", "evidence", "TF-109.jsonl"))
	if err != nil {
		t.Fatalf("failed verification evidence missing: %v", err)
	}
	if !strings.Contains(string(evidence), `"outcome":"FAILED"`) || !strings.HasSuffix(string(evidence), "\n") {
		t.Fatalf("unexpected evidence: %s", evidence)
	}
}

func TestStatusReportsStableReadOnlyTaskCounts(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	if output, err := runCLI(t, binary, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, output)
	}
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		placeholder := filepath.Join(root, "tasks", state, ".gitkeep")
		if err := os.WriteFile(placeholder, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := runCLI(t, binary, root, "status"); err != nil || string(output) != "inbox: 0\nready: 0\nactive: 0\nfailed: 0\narchive: 0\n" {
		t.Fatalf("empty status = %q, err=%v", output, err)
	}

	for _, item := range []struct{ state, filename, contents string }{
		{"inbox", "TF-101-proposal.md", "# TF-101: Proposal\nAnything goes.\n"},
		{"ready", "TF-102-ready.md", validTask("TF-102")},
		{"active", "TF-103-active.md", activeTask(t, root, "TF-103")},
		{"failed", "TF-104-failed.md", validTask("TF-104")},
		{"archive", "TF-105-archived.md", validTask("TF-105")},
	} {
		path := filepath.Join(root, "tasks", item.state, item.filename)
		if err := os.WriteFile(path, []byte(item.contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := "inbox: 1\nready: 1\nactive: 1\nfailed: 1\narchive: 1\n"
	before := taskTreeHashes(t, root)
	first, err := runCLI(t, binary, root, "status")
	if err != nil || string(first) != want {
		t.Fatalf("populated status = %q, err=%v; want %q", first, err, want)
	}
	second, err := runCLI(t, binary, root, "status")
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("repeated status differs: first=%q second=%q err=%v", first, second, err)
	}
	if after := taskTreeHashes(t, root); !equalTaskHashes(before, after) {
		t.Fatal("status modified task files")
	}
}

func TestStatusRejectsMissingStateDirectoryAndInvalidTask(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	if output, err := runCLI(t, binary, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, output)
	}
	missing := filepath.Join(root, "tasks", "failed")
	if err := os.RemoveAll(missing); err != nil {
		t.Fatal(err)
	}
	output, err := runCLI(t, binary, root, "status")
	if processExitCode(err) == 0 || !strings.Contains(string(output), missing) {
		t.Fatalf("missing directory should fail with its path: err=%v output=%s", err, output)
	}
	if err := os.Mkdir(missing, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(root, "tasks", "ready", "TF-107-broken.md")
	if err := os.WriteFile(bad, []byte("# TF-107: Broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err = runCLI(t, binary, root, "status")
	if processExitCode(err) == 0 || !strings.Contains(string(output), "tasks/ready/TF-107-broken.md") || !strings.Contains(string(output), "Goal") {
		t.Fatalf("invalid task should fail with a path-specific diagnostic: err=%v output=%s", err, output)
	}
}

func validTask(id string) string {
	return "# " + id + ": Example\n\n## Goal\n\nDo it.\n\n## Dependencies\n\nNone\n\n## Scope\n\nImplement.\n\n## Constraints\n\nKeep it small.\n\n## Success criteria\n\n### C1: It works\n\nCheck: go test ./...\n\n## Verification\n\nRun check.\n"
}

func activeTask(t *testing.T, root, id string) string {
	t.Helper()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return validTask(id) + "\n## Claim\n\nOwner: test\nBranch: feature/test\nWorktree: " + filepath.Join(canonicalRoot, ".taskfactory", "worktrees", id) + "\nBase commit: 0123456789abcdef0123456789abcdef01234567\nStarted at: 2026-10-08T12:00:00Z\n"
}

func taskTreeHashes(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	result := make(map[string][32]byte)
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		entries, err := os.ReadDir(filepath.Join(root, "tasks", state))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.Name() == ".gitkeep" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, "tasks", state, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			result[state+"/"+entry.Name()] = sha256.Sum256(data)
		}
	}
	return result
}

func equalTaskHashes(left, right map[string][32]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for path, hash := range left {
		if right[path] != hash {
			return false
		}
	}
	return true
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
	cmd := exec.Command("git", "init", "--quiet", "--initial-branch=main", root)
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

func TestValidatePathsSelectsFilesAndExitCodes(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	if output, err := runCLI(t, binary, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, output)
	}
	valid := "# TF-101: Example\n\n## Goal\n\nDo it.\n\n## Dependencies\n\nNone\n\n## Scope\n\nImplement.\n\n## Constraints\n\nKeep it small.\n\n## Success criteria\n\n### C1: It works\n\nCheck: go test ./...\n\n## Verification\n\nRun check.\n"
	readyPath := filepath.Join(root, "tasks", "ready", "TF-101-example.md")
	archivePath := filepath.Join(root, "tasks", "archive", "TF-102-old.md")
	inboxPath := filepath.Join(root, "tasks", "inbox", "TF-103-idea.md")
	if err := os.WriteFile(readyPath, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, []byte("# TF-102: Broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Only the archive is broken: the default inbox and ready selection passes.
	if output, err := runCLI(t, binary, root, "validate"); err != nil {
		t.Fatalf("no arguments with broken archive: %v\n%s", err, output)
	}
	output, err := runCLI(t, binary, root, "validate", filepath.Join("tasks", "archive"))
	if processExitCode(err) != 1 || !strings.Contains(string(output), "tasks/archive/TF-102-old.md") {
		t.Fatalf("archive folder exit=%d output=%s", processExitCode(err), output)
	}
	if output, err := runCLI(t, binary, root, "validate", "tasks/ready/TF-101-example.md"); err != nil {
		t.Fatalf("valid file with unrelated broken archive: %v\n%s", err, output)
	}

	if err := os.WriteFile(inboxPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readyPath, []byte(strings.Replace(valid, "## Goal\n\nDo it.\n\n", "", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err = runCLI(t, binary, root, "validate")
	if processExitCode(err) != 1 || !strings.Contains(string(output), "tasks/ready/TF-101-example.md") || !strings.Contains(string(output), "tasks/inbox/TF-103-idea.md") || strings.Contains(string(output), "tasks/archive/TF-102-old.md") {
		t.Fatalf("no arguments should report inbox and ready only: exit=%d output=%s", processExitCode(err), output)
	}
	output, err = runCLI(t, binary, root, "validate", "tasks/ready/TF-101-example.md", "tasks/archive/TF-102-old.md")
	if processExitCode(err) != 1 || !strings.Contains(string(output), "TF-101-example.md") || !strings.Contains(string(output), "TF-102-old.md") {
		t.Fatalf("two selected files should both be reported: exit=%d output=%s", processExitCode(err), output)
	}
	output, err = runCLI(t, binary, root, "validate", "tasks")
	if processExitCode(err) != 1 || !strings.Contains(string(output), "TF-103-idea.md") {
		t.Fatalf("whole tree exit=%d output=%s", processExitCode(err), output)
	}

	for _, args := range [][]string{
		{"validate", "../outside.md"},
		{"validate", root},
		{"validate", filepath.Join("tasks", "ready", "notes.txt")},
		{"validate", filepath.Join("tasks", "missing")},
	} {
		if output, err := runCLI(t, binary, root, args...); processExitCode(err) != 2 {
			t.Errorf("%v exit=%d, want 2: %s", args, processExitCode(err), output)
		}
	}
}

func TestVerifyPathArgumentSuggestsValidate(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	output, err := runCLI(t, binary, root, "verify", "tasks/inbox/x.md")
	if processExitCode(err) != 2 || !strings.Contains(string(output), "did you mean `taskfactory validate <path>`?") {
		t.Fatalf("verify path exit=%d output=%s", processExitCode(err), output)
	}
}

// verifySummaryProject creates a claimed task whose only criterion is check and
// returns the project root and the CLI binary path.
func verifySummaryProject(t *testing.T, id, check string) (string, string) {
	t.Helper()
	binary := buildCLI(t)
	root := initGitProject(t)
	if output, err := runCLI(t, binary, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, output)
	}
	// Replace the default Go worker command with a trivial one that always passes.
	configPath := filepath.Join(root, ".taskfactory", "config.toml")
	cfg, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg = []byte(strings.Replace(string(cfg), `worker = ["go test ./..."]`, `worker = ["true"]`, 1))
	if err := os.WriteFile(configPath, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	task := strings.Replace(validTask(id), "Check: go test ./...", "Check: "+check, 1)
	if err := os.WriteFile(filepath.Join(root, "tasks", "ready", id+"-summary.md"), []byte(task), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "-m", "fixture"}} {
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	if output, err := runCLI(t, binary, root, "claim", id, "--owner", "worker"); err != nil {
		t.Fatalf("claim: %v\n%s", err, output)
	}
	return root, binary
}

// runVerifySummary runs verify and returns stdout only, plus the exit code.
func runVerifySummary(t *testing.T, binary, root, id string) (string, int) {
	t.Helper()
	cmd := exec.Command(binary, "verify", id)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	stdout, err := cmd.Output()
	return string(stdout), processExitCode(err)
}

func TestSummaryPass(t *testing.T) {
	root, binary := verifySummaryProject(t, "TF-111", "true")
	stdout, code := runVerifySummary(t, binary, root, "TF-111")
	if code != 0 {
		t.Fatalf("verify exit = %d, want 0; stdout: %s", code, stdout)
	}
	if want := "verify TF-111: PASS (2 checks)\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestSummaryFail(t *testing.T) {
	root, binary := verifySummaryProject(t, "TF-112", "false")
	stdout, code := runVerifySummary(t, binary, root, "TF-112")
	if code != 1 {
		t.Fatalf("verify exit = %d, want 1; stdout: %s", code, stdout)
	}
	if want := "verify TF-112: FAIL (1 of 2 checks failed)\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

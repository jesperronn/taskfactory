package workprompt

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testID     = "TF-055"
	testBranch = "task/TF-055-impl"
	testConfig = `protocol_version = 1

[workers]
max_parallel = 4

[git]
use_worktrees = true
worktree_root = ".taskfactory/worktrees"
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = ["go test ./..."]
integration = ["go test ./...", "go vet ./..."]
`
	testInstructions = "# Worker verification instructions\n\nRun bin/test and bin/lint.\n"
)

// git runs a Git command in dir with global and system configuration disabled.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=Test Worker", "GIT_AUTHOR_EMAIL=worker@example.invalid",
		"GIT_COMMITTER_NAME=Test Worker", "GIT_COMMITTER_EMAIL=worker@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// taskContract returns a task file for id. With claim set it appends a Claim
// block for the given worktree and base commit; dropBase omits the Base commit
// line to make the block incomplete.
func taskContract(id, worktree, base string, claim, dropBase bool) string {
	body := "# " + id + ": Example task\n\n" +
		"## Goal\n\nMake the example change.\n\n" +
		"## Dependencies\n\nNone\n\n" +
		"## Scope\n\nTouch only the example file.\n\n" +
		"## Constraints\n\nUse only the standard library.\n\n" +
		"## Success criteria\n\n### C1: Example passes\n\nCheck: go test ./...\n\n" +
		"## Verification\n\nRun the check from the repository root.\n"
	if claim {
		body += "\n## Claim\n\nOwner: test-worker\nBranch: " + testBranch + "\nWorktree: " + worktree + "\n"
		if !dropBase {
			body += "Base commit: " + base + "\n"
		}
		body += "Started at: 2026-10-09T10:00:00Z\n"
	}
	return body
}

// fixture is a temporary repository on main with an active, claimed task whose
// worktree is a real Git worktree holding the worker instructions.
type fixture struct {
	root     string
	worktree string
	base     string
	taskFile string
	task     string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "config", "commit.gpgsign", "false")
	git(t, root, "config", "user.name", "Test Worker")
	git(t, root, "config", "user.email", "worker@example.invalid")
	write(t, filepath.Join(root, ".taskfactory", "config.toml"), testConfig)
	write(t, filepath.Join(root, "docs", "worker-instructions.md"), testInstructions)
	write(t, filepath.Join(root, "example.txt"), "base\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "init")
	base := git(t, root, "rev-parse", "refs/heads/main")

	worktree := filepath.Join(root, ".taskfactory", "worktrees", testID)
	if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, root, "worktree", "add", "-q", worktree, "-b", testBranch, "main")

	f := fixture{root: root, worktree: worktree, base: base}
	f.task = taskContract(testID, worktree, base, true, false)
	f.taskFile = filepath.Join(root, "tasks", "active", testID+"-example.md")
	write(t, f.taskFile, f.task)
	return f
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildReturnsClaimFieldsAndVerbatimTask(t *testing.T) {
	f := newFixture(t)
	p, err := Build(f.root, testID)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if p.Worktree != f.worktree || p.Branch != testBranch || p.BaseCommit != f.base {
		t.Fatalf("Build fields = %+v, want worktree %s branch %s base %s", p, f.worktree, testBranch, f.base)
	}
	if !strings.Contains(p.Text, f.task) {
		t.Fatalf("prompt does not contain the task file verbatim")
	}
	if !strings.Contains(p.Text, testInstructions) {
		t.Fatalf("prompt does not contain the worktree worker instructions")
	}
	header := strings.Index(p.Text, "Task: "+testID+"\nBranch: "+testBranch+"\nWorktree: "+f.worktree)
	task := strings.Index(p.Text, f.task)
	instr := strings.Index(p.Text, testInstructions)
	rules := strings.Index(p.Text, "----- RULES -----")
	if !(header >= 0 && header < task && task < instr && instr < rules) {
		t.Fatalf("prompt sections out of order: header=%d task=%d instructions=%d rules=%d", header, task, instr, rules)
	}
}

func TestBuildDeterministicReadsFilesOnly(t *testing.T) {
	f := newFixture(t)
	before := snapshot(t, f.root)
	first, err := Build(f.root, testID)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	second, err := Build(f.root, testID)
	if err != nil {
		t.Fatalf("second Build: %v", err)
	}
	if first != second {
		t.Fatalf("Build returned different results for the same inputs")
	}
	if after := snapshot(t, f.root); after != before {
		t.Fatalf("Build changed the file tree:\nbefore: %s\nafter:  %s", before, after)
	}
}

// snapshot lists every path under root with its size and modification time,
// excluding the .git directory, so a write by Build is detected.
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" && d.IsDir() {
			return filepath.SkipDir
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s %d %s\n", path, info.Size(), info.ModTime().UTC().Format(time.RFC3339Nano))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestBuildRefusesMissingUnclaimedOrIncompleteTask(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		f := newFixture(t)
		if _, err := Build(f.root, "TF-099"); err == nil {
			t.Fatal("Build accepted a task that is not in tasks/active")
		}
	})
	t.Run("not active", func(t *testing.T) {
		f := newFixture(t)
		write(t, filepath.Join(f.root, "tasks", "inbox", "TF-077-other.md"), taskContract("TF-077", "", "", false, false))
		if _, err := Build(f.root, "TF-077"); err == nil {
			t.Fatal("Build accepted an inbox task")
		}
	})
	t.Run("unclaimed", func(t *testing.T) {
		f := newFixture(t)
		write(t, f.taskFile, taskContract(testID, "", "", false, false))
		if _, err := Build(f.root, testID); err == nil {
			t.Fatal("Build accepted an active task without a Claim block")
		}
	})
	t.Run("incomplete claim", func(t *testing.T) {
		f := newFixture(t)
		write(t, f.taskFile, taskContract(testID, f.worktree, f.base, true, true))
		if _, err := Build(f.root, testID); err == nil {
			t.Fatal("Build accepted a Claim block without Base commit")
		}
	})
	t.Run("missing instructions", func(t *testing.T) {
		f := newFixture(t)
		if err := os.Remove(filepath.Join(f.worktree, "docs", "worker-instructions.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := Build(f.root, testID); err == nil {
			t.Fatal("Build accepted a worktree without worker instructions")
		}
	})
}

func TestRulesForbidIntegrateFailPushAndSigning(t *testing.T) {
	f := newFixture(t)
	p, err := Build(f.root, testID)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	rules := p.Text[strings.Index(p.Text, "----- RULES -----"):]
	for _, want := range []string{
		"Do not run integrate, fail or promote, and do not push.",
		"Commit the result with plain git commit.",
		"Never change Git signing configuration.",
		"bin/test and bin/lint",
		"report each exit code",
		"stop and report a blocker",
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("rules block lacks %q", want)
		}
	}
	for _, banned := range []string{"gpgsign", "no-gpg-sign", "no-verify", "git push", "run integrate and"} {
		if strings.Contains(rules, banned) {
			t.Fatalf("rules block contains %q", banned)
		}
	}
}

func TestLogPathLayoutAndOpenLogNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 9, 12, 34, 56, 0, time.FixedZone("CEST", 2*60*60))
	want := filepath.Join(root, ".taskfactory", "logs", testID, "claude-20261009T103456Z.log")
	path := LogPath(root, testID, "claude", at)
	if path != want {
		t.Fatalf("LogPath = %s, want %s", path, want)
	}
	h := Header{TaskID: testID, Adapter: "claude", Model: "fixture-model", Timeout: 30 * time.Minute, Start: at}
	file, err := OpenLog(path, h)
	if err != nil {
		t.Fatalf("OpenLog: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("log mode = %v, want 0644", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wantHeader := "TaskFactory worker log\nTask: TF-055\nAdapter: claude\nModel: fixture-model\nTimeout: 30m0s\nStarted at: 2026-10-09T10:34:56Z\n\n"
	if string(data) != wantHeader {
		t.Fatalf("log header =\n%q\nwant\n%q", data, wantHeader)
	}
	if _, err := OpenLog(path, h); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("second OpenLog error = %v, want refusal to overwrite", err)
	}
	bad := []Header{
		{TaskID: testID, Adapter: "bash", Model: "m", Timeout: time.Minute, Start: at},
		{TaskID: testID, Adapter: "", Model: "m", Timeout: time.Minute, Start: at},
		{TaskID: testID, Adapter: "../claude", Model: "m", Timeout: time.Minute, Start: at},
		{TaskID: "TF-056", Adapter: "claude", Model: "m", Timeout: time.Minute, Start: at},
		{TaskID: testID, Adapter: "claude", Model: "m\nInjected: x", Timeout: time.Minute, Start: at},
	}
	for _, b := range bad {
		if _, err := OpenLog(path, b); err == nil {
			t.Fatalf("OpenLog accepted header %+v", b)
		}
	}
	if _, err := OpenLog(LogPath(root, testID, "pi", at), Header{TaskID: testID, Adapter: "claude", Model: "m", Timeout: time.Minute, Start: at}); err == nil {
		t.Fatal("OpenLog accepted a path for a different adapter")
	}
}

func TestNoSecretsInPromptOrLogHeader(t *testing.T) {
	const secret = "sk-test-SENTINEL-do-not-print"
	t.Setenv("ANTHROPIC_AUTH_TOKEN", secret)
	f := newFixture(t)
	p, err := Build(f.root, testID)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if strings.Contains(p.Text, secret) {
		t.Fatal("prompt contains the ANTHROPIC_AUTH_TOKEN value")
	}
	at := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	file, err := OpenLog(LogPath(f.root, testID, "claude", at), Header{TaskID: testID, Adapter: "claude", Model: "m", Timeout: time.Minute, Start: at})
	if err != nil {
		t.Fatalf("OpenLog: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(LogPath(f.root, testID, "claude", at))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatal("log header contains the ANTHROPIC_AUTH_TOKEN value")
	}
}

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// runWithEnv is runSplit with extra environment variables. The directory is
// always a scratch directory, never the real checkout.
func runWithEnv(t *testing.T, binary, dir string, env []string, args ...string) cliResult {
	t.Helper()
	if dir == "" {
		t.Fatal("refusing to run the CLI without an explicit scratch directory")
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "NO_COLOR=1"), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return cliResult{stdout.String(), stderr.String(), processExitCode(err)}
}

// sharedFixtureDir holds fixtures shared across tests; TestMain removes it.
// sharedFixtureBases maps a test that is building a shared scenario fixture to
// the directory initGitProject must use for that test only, so parallel tests
// building their own projects are never redirected.
var (
	sharedFixtureDir   string
	sharedBasesMu      sync.Mutex
	sharedFixtureBases = map[*testing.T]string{}
)

// fixtureConfig is the project configuration the dry-run fixtures commit.
const fixtureConfig = "protocol_version = 1\n\n[workers]\nmax_parallel = 4\n\n[git]\nuse_worktrees = true\nworktree_root = \".taskfactory/worktrees\"\nintegration_strategy = \"ff-only\"\n\n[integration]\nstop_on_main_failure = true\n\n[verification]\nworker = [\"true\"]\nintegration = [\"true\"]\nmain = [\"touch main.ran\"]\n"

// stateFixture returns a temporary repository with task TF-<id> in the given
// state: inbox, ready, active, verified (a worker commit and a passing verify)
// or failed (claimed, then failed). The task's own check is check.
func stateFixture(t *testing.T, binary, state, id, check string) string {
	t.Helper()
	root := nextStepProject(t, binary, id, check)
	if err := os.WriteFile(filepath.Join(root, ".taskfactory", "config.toml"), []byte(fixtureConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "commit", "--no-gpg-sign", "-q", "-am", "main check")
	step := func(args ...string) {
		t.Helper()
		if res := runSplit(t, binary, root, args...); res.code != 0 {
			t.Fatalf("%v: %+v", args, res)
		}
	}
	task := "TF-" + id
	if state == "inbox" {
		return root
	}
	step("promote", task)
	if state == "ready" {
		return root
	}
	step("claim", task, "--owner", "w")
	switch state {
	case "active":
		return root
	case "failed":
		step("fail", task, "--outcome", "BLOCKED", "--reason", "fixture")
		return root
	case "verified":
		worktree := filepath.Join(root, ".taskfactory", "worktrees", task)
		if err := os.WriteFile(filepath.Join(worktree, "feature.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		mustGit(t, worktree, "add", "feature.txt")
		mustGit(t, worktree, "commit", "--no-gpg-sign", "-q", "-m", "feature")
		step("verify", task)
		return root
	}
	t.Fatalf("unknown fixture state %q", state)
	return ""
}

// snapshot captures everything a dry run must leave alone: Git status and
// worktrees, every ref, and every file outside .git with its content hash.
func snapshot(t *testing.T, root string) []string {
	t.Helper()
	// One git process per snapshot: status, worktrees and refs together.
	script := `git status --short --untracked-files=all && echo --worktrees && git worktree list && echo --refs && git for-each-ref`
	cmd := exec.Command("sh", "-c", script)
	cmd.Dir = root
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git snapshot: %v\n%s", err, data)
	}
	out := []string{"git\n" + string(data)}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			return filepath.SkipDir
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			out = append(out, "dir "+rel)
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out = append(out, fmt.Sprintf("file %s %x", rel, sha256.Sum256(data)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out[1:])
	return out
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	data, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, data)
	}
	return string(data)
}

// Dry-run scenarios only read their fixture (every test asserts a dry run
// changes nothing), so each scenario's fixture is built once per test binary.
// Each scenario has its own sync.Once, so different scenarios build
// concurrently while tests of one scenario wait for its single build.
type scenarioEntry struct {
	once sync.Once
	root string
}

var (
	scenarioMu      sync.Mutex
	scenarioEntries = map[string]*scenarioEntry{}
	sharedDirOnce   sync.Once
)

func scenarioRoot(t *testing.T, binary string, sc dryScenario) string {
	t.Helper()
	scenarioMu.Lock()
	entry, ok := scenarioEntries[sc.name]
	if !ok {
		entry = &scenarioEntry{}
		scenarioEntries[sc.name] = entry
	}
	scenarioMu.Unlock()
	entry.once.Do(func() {
		sharedDirOnce.Do(func() {
			dir, err := os.MkdirTemp("", "taskfactory-dry")
			if err != nil {
				t.Fatal(err)
			}
			sharedFixtureDir = dir
		})
		base, err := os.MkdirTemp(sharedFixtureDir, "s")
		if err != nil {
			t.Fatal(err)
		}
		sharedBasesMu.Lock()
		sharedFixtureBases[t] = base
		sharedBasesMu.Unlock()
		defer func() {
			sharedBasesMu.Lock()
			delete(sharedFixtureBases, t)
			sharedBasesMu.Unlock()
		}()
		entry.root = sc.setup(t, binary)
	})
	if entry.root == "" {
		t.Fatalf("shared fixture for scenario %q failed to build", sc.name)
	}
	return entry.root
}

// dryScenario is one mutating command run with --dry-run on a fixture.
type dryScenario struct {
	name  string
	setup func(t *testing.T, binary string) string
	args  []string
	would []string // substrings, each of which some would: line must contain
}

func dryScenarios() []dryScenario {
	fixture := func(state, id, check string) func(*testing.T, string) string {
		return func(t *testing.T, binary string) string { return stateFixture(t, binary, state, id, check) }
	}
	return []dryScenario{
		{"init", func(t *testing.T, binary string) string { return initGitProject(t) },
			[]string{"init"}, []string{"create .taskfactory/config.toml", "create tasks/ready", "create .gitignore"}},
		{"promote", fixture("inbox", "911", "true"),
			[]string{"promote", "TF-911"}, []string{"move task to tasks/ready", "commit"}},
		{"claim", fixture("ready", "912", "true"),
			[]string{"claim", "TF-912", "--owner", "w"}, []string{"move task to tasks/active", "add worktree /", "/.taskfactory/worktrees/TF-912"}},
		{"fail", fixture("active", "913", "true"),
			[]string{"fail", "TF-913", "--outcome", "BLOCKED", "--reason", "why"}, []string{"move task to tasks/failed", "commit"}},
		{"requeue", fixture("failed", "914", "true"),
			[]string{"requeue", "TF-914", "--discard-claim"}, []string{"move task to tasks/ready", "commit", "remove worktree", "remove branch task/TF-914"}},
		{"verify", fixture("active", "915", "touch ran.marker"),
			[]string{"verify", "TF-915"}, []string{"run touch ran.marker", "append evidence"}},
		{"integrate", fixture("verified", "916", "true"),
			[]string{"integrate", "TF-916"}, []string{"run true", "fast-forward main", "archive"}},
		{"check-main", fixture("ready", "917", "true"),
			[]string{"check-main"}, []string{"run touch main.ran"}},
		{"work", fixture("active", "918", "true"),
			[]string{"work", "TF-918", "--adapter", "pi", "--model", "m"}, []string{"launch pi", "write log"}},
	}
}

// runDryScenario runs the scenario with --dry-run, in each flag position, and
// returns the result of the last run after checking that nothing changed.
func runDryScenario(t *testing.T, binary string, sc dryScenario) cliResult {
	t.Helper()
	root := scenarioRoot(t, binary, sc)
	// A fake adapter that records any launch, put first on PATH.
	bin := t.TempDir()
	marker := filepath.Join(bin, "adapter-launched")
	script := "#!/bin/sh\ntouch " + marker + "\nexit 0\n"
	for _, name := range []string{"pi", "omp", "claude"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}
	before := snapshot(t, root)
	var last cliResult
	for _, position := range []string{"after", "before"} {
		args := append([]string(nil), sc.args...)
		if position == "after" {
			args = append(args, "--dry-run")
		} else {
			args = append([]string{args[0], "--dry-run"}, args[1:]...)
		}
		last = runWithEnv(t, binary, root, env, args...)
		if last.code != 0 {
			t.Fatalf("%v: exit %d\nstdout: %s\nstderr: %s", args, last.code, last.stdout, last.stderr)
		}
		if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
			t.Fatalf("%v changed the project:\nbefore: %q\nafter:  %q", args, before, after)
		}
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("%v launched an adapter", sc.args)
	}
	return last
}

func scenarioNamed(t *testing.T, name string) dryScenario {
	t.Helper()
	for _, sc := range dryScenarios() {
		if sc.name == name {
			return sc
		}
	}
	t.Fatalf("no scenario %q", name)
	return dryScenario{}
}

func assertWould(t *testing.T, sc dryScenario, res cliResult) {
	t.Helper()
	for _, want := range sc.would {
		found := false
		for _, line := range lines(res.stdout) {
			if strings.HasPrefix(line, "would: ") && strings.Contains(line, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no would: line contains %q:\n%s", sc.name, want, res.stdout)
		}
	}
}

func TestCommonFlagsPlacement(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		rest []string
		want commonFlags
	}{
		{[]string{"TF-1", "--dry-run"}, []string{"TF-1"}, commonFlags{DryRun: true}},
		{[]string{"--dry-run", "TF-1"}, []string{"TF-1"}, commonFlags{DryRun: true}},
		{[]string{"--owner", "w", "--verbose", "TF-1"}, []string{"--owner", "w", "TF-1"}, commonFlags{Verbose: true}},
		{[]string{"TF-1", "--outcome", "FAILED", "--dry-run", "--verbose"}, []string{"TF-1", "--outcome", "FAILED"}, commonFlags{Verbose: true, DryRun: true}},
		{nil, []string{}, commonFlags{}},
	}
	for _, c := range cases {
		rest, got, err := extractCommonFlags(c.args)
		if err != nil || !reflect.DeepEqual(rest, c.rest) || got != c.want {
			t.Errorf("extractCommonFlags(%q) = %q, %+v, %v; want %q, %+v", c.args, rest, got, err, c.rest, c.want)
		}
	}

	binary := buildCLI(t)
	root := stateFixture(t, binary, "inbox", "921", "true")
	for _, args := range [][]string{
		{"promote", "--dry-run", "TF-921"},
		{"promote", "TF-921", "--dry-run"},
		{"promote", "--verbose", "TF-921", "--dry-run"},
	} {
		res := runSplit(t, binary, root, args...)
		if res.code != 0 || !strings.HasSuffix(res.stdout, "dry run: nothing changed\n") {
			t.Errorf("%v: %+v", args, res)
		}
	}
	// Global flags stay --help and --version.
	res := runSplit(t, binary, root, "--dry-run", "promote", "TF-921")
	if res.code != 2 || !strings.Contains(res.stderr, "flag provided but not defined") {
		t.Errorf("global --dry-run: %+v", res)
	}
}

func TestCommonFlagsRepeatAndValueForm(t *testing.T) {
	t.Parallel()
	rest, got, err := extractCommonFlags([]string{"--dry-run", "TF-1", "--dry-run", "--verbose", "--verbose"})
	if err != nil || !reflect.DeepEqual(rest, []string{"TF-1"}) || !got.DryRun || !got.Verbose {
		t.Errorf("repeated flags: %q %+v %v", rest, got, err)
	}
	binary := buildCLI(t)
	scratch := t.TempDir()
	for _, args := range [][]string{
		{"status", "--dry-run=true"},
		{"promote", "TF-1", "--verbose=1"},
		{"init", "--dry-run=false"},
		{"work", "TF-1", "--dry-run="},
	} {
		res := runSplit(t, binary, scratch, args...)
		if res.code != 2 || !strings.Contains(res.stderr, "takes no value") {
			t.Errorf("%v: %+v", args, res)
		}
	}
	root := stateFixture(t, binary, "inbox", "922", "true")
	if res := runSplit(t, binary, root, "promote", "--dry-run", "TF-922", "--dry-run", "--verbose"); res.code != 0 {
		t.Errorf("repeated flag in CLI: %+v", res)
	}
}

func TestHelpListsVerboseAndDryRun(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	scratch := t.TempDir()
	for _, command := range commandSummaries {
		if command.name == "completion" {
			continue // exempt from the common flags by contract
		}
		res := runSplit(t, binary, scratch, command.name, "--help")
		if res.code != 0 {
			t.Fatalf("%s --help: %+v", command.name, res)
		}
		flagsAt := strings.Index(res.stdout, "\nFlags:\n")
		exitAt := strings.Index(res.stdout, "\nExit codes:")
		if flagsAt < 0 || exitAt < flagsAt {
			t.Fatalf("%s help has no Flags block:\n%s", command.name, res.stdout)
		}
		block := res.stdout[flagsAt:exitAt]
		for _, flag := range []string{"  --verbose ", "  --dry-run "} {
			if !strings.Contains(block, flag) {
				t.Errorf("%s Flags block lacks %q:\n%s", command.name, flag, block)
			}
		}
		sentence := "With --dry-run nothing changes."
		if command.name == "status" || command.name == "validate" {
			sentence = "--dry-run is accepted and has no effect, because this command only reads."
		}
		if !strings.Contains(res.stdout, sentence) {
			t.Errorf("%s help lacks %q", command.name, sentence)
		}
	}
}

func TestDryRunInfoCommandsNoOp(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := stateFixture(t, binary, "ready", "923", "true")
	before := snapshot(t, root)
	for _, args := range [][]string{{"status"}, {"validate"}, {"validate", "tasks"}} {
		normal := runSplit(t, binary, root, args...)
		dry := runSplit(t, binary, root, append(append([]string(nil), args...), "--dry-run")...)
		if normal.code != 0 || dry.code != 0 {
			t.Fatalf("%v: normal %+v dry %+v", args, normal, dry)
		}
		wantStdout := strings.Join(withoutNext(lines(normal.stdout)), "\n") + "\ndry run: nothing changed\n"
		if dry.stdout != wantStdout {
			t.Errorf("%v dry stdout = %q, want %q", args, dry.stdout, wantStdout)
		}
		if strings.Contains(dry.stdout, "would: ") || countNext(dry.stdout) != 0 {
			t.Errorf("%v dry stdout has would/next lines: %q", args, dry.stdout)
		}
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Errorf("info commands changed the project")
	}
}

func withoutNext(in []string) []string {
	var out []string
	for _, line := range in {
		if !strings.HasPrefix(line, "next: ") {
			out = append(out, line)
		}
	}
	return out
}

func TestDryRunInitPromoteClaimChangeNothing(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	for _, name := range []string{"init", "promote", "claim"} {
		sc := scenarioNamed(t, name)
		res := runDryScenario(t, binary, sc)
		assertWould(t, sc, res)
	}
	// init with a path applies the dry run to the path form too.
	root := initGitProject(t)
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	res := runSplit(t, binary, root, "init", sub, "--dry-run")
	if res.code != 0 || !strings.Contains(res.stdout, "would: create .taskfactory/config.toml") {
		t.Errorf("init <path> --dry-run: %+v", res)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("init <path> --dry-run changed the project")
	}
}

func TestDryRunFailRequeueChangeNothing(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	for _, name := range []string{"fail", "requeue"} {
		sc := scenarioNamed(t, name)
		assertWould(t, sc, runDryScenario(t, binary, sc))
	}
}

func TestDryRunVerifyWritesNoEvidence(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	sc := scenarioNamed(t, "verify")
	root := scenarioRoot(t, binary, sc)
	res := runSplit(t, binary, root, "verify", "TF-915", "--dry-run")
	if res.code != 0 {
		t.Fatalf("verify --dry-run: %+v", res)
	}
	assertWould(t, sc, res)
	worktree := filepath.Join(root, ".taskfactory", "worktrees", "TF-915")
	for _, path := range []string{
		filepath.Join(worktree, "ran.marker"),
		filepath.Join(root, ".taskfactory", "evidence"),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s exists after a dry run", path)
		}
	}
	runDryScenario(t, binary, sc)
}

func TestDryRunIntegrateCheckMainChangeNothing(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	sc := scenarioNamed(t, "integrate")
	root := scenarioRoot(t, binary, sc)
	mainBefore := gitOutput(t, root, "rev-parse", "main")
	branchBefore := gitOutput(t, root, "rev-parse", "task/TF-916")
	res := runSplit(t, binary, root, "integrate", "TF-916", "--dry-run")
	if res.code != 0 {
		t.Fatalf("integrate --dry-run: %+v", res)
	}
	assertWould(t, sc, res)
	if gitOutput(t, root, "rev-parse", "main") != mainBefore || gitOutput(t, root, "rev-parse", "task/TF-916") != branchBefore {
		t.Error("integrate --dry-run moved a ref")
	}
	for _, path := range []string{".taskfactory/integration.lock", ".taskfactory/integration-evidence"} {
		if _, err := os.Stat(filepath.Join(root, path)); err == nil {
			t.Errorf("%s exists after a dry run", path)
		}
	}
	runDryScenario(t, binary, sc)

	cm := scenarioNamed(t, "check-main")
	cmRoot := scenarioRoot(t, binary, cm)
	res = runSplit(t, binary, cmRoot, "check-main", "--dry-run")
	if res.code != 0 {
		t.Fatalf("check-main --dry-run: %+v", res)
	}
	assertWould(t, cm, res)
	if _, err := os.Stat(filepath.Join(cmRoot, "main.ran")); err == nil {
		t.Error("check-main --dry-run ran the main check")
	}
	runDryScenario(t, binary, cm)
}

func TestDryRunWorkNeverLaunchesAdapter(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	sc := scenarioNamed(t, "work")
	res := runDryScenario(t, binary, sc) // fails if the fake adapter ran
	assertWould(t, sc, res)
	root := scenarioRoot(t, binary, sc)
	runSplit(t, binary, root, "work", "TF-918", "--adapter", "pi", "--model", "m", "--dry-run")
	if _, err := os.Stat(filepath.Join(root, ".taskfactory", "logs")); err == nil {
		t.Error("work --dry-run wrote a log directory")
	}
}

func TestDryRunLeavesGitStatusAndWorktreesUnchanged(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	for _, sc := range dryScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			root := scenarioRoot(t, binary, sc)
			status := gitOutput(t, root, "status", "--short", "--untracked-files=all")
			worktrees := gitOutput(t, root, "worktree", "list")
			res := runSplit(t, binary, root, append(append([]string(nil), sc.args...), "--dry-run")...)
			if res.code != 0 {
				t.Fatalf("%+v", res)
			}
			if got := gitOutput(t, root, "status", "--short", "--untracked-files=all"); got != status {
				t.Errorf("git status changed:\nbefore: %q\nafter:  %q", status, got)
			}
			if got := gitOutput(t, root, "worktree", "list"); got != worktrees {
				t.Errorf("git worktree list changed:\nbefore: %q\nafter:  %q", worktrees, got)
			}
		})
	}
}

func TestDryRunOutputFormat(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	for _, sc := range dryScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			root := scenarioRoot(t, binary, sc)
			res := runSplit(t, binary, root, append(append([]string(nil), sc.args...), "--dry-run")...)
			if res.code != 0 || res.stderr != "" {
				t.Fatalf("%+v", res)
			}
			out := lines(res.stdout)
			if len(out) < 2 || out[len(out)-1] != "dry run: nothing changed" {
				t.Fatalf("stdout does not end with the closing line: %q", res.stdout)
			}
			for _, line := range out[:len(out)-1] {
				if !strings.HasPrefix(line, "would: ") {
					t.Errorf("line %q does not start with would: ", line)
				}
			}
			if countNext(res.stdout) != 0 {
				t.Errorf("dry run printed a next-step hint: %q", res.stdout)
			}
		})
	}
}

func TestDryRunRefusesLikeRealCommand(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := stateFixture(t, binary, "inbox", "924", "true")
	cases := [][]string{
		{"promote", "TF-999"},
		{"promote"},
		{"promote", "bad"},
		{"claim", "TF-924", "--owner", "w"}, // inbox, not ready
		{"claim", "TF-999", "--owner", "w"},
		{"fail", "TF-999", "--outcome", "FAILED", "--reason", "x"},
		{"fail", "TF-924"},
		{"requeue", "TF-999"},
		{"requeue", "TF-924", "--to", "active"},
		{"verify", "TF-999"},
		{"integrate", "TF-999"},
		{"work", "TF-999", "--adapter", "pi", "--model", "m"},
		{"work", "TF-999"},
		{"validate", "../outside.md"},
	}
	for _, args := range cases {
		real := runSplit(t, binary, root, args...)
		dry := runSplit(t, binary, root, append(append([]string(nil), args...), "--dry-run")...)
		if real.code == 0 {
			t.Fatalf("%v unexpectedly succeeded: %+v", args, real)
		}
		if dry.code != real.code {
			t.Errorf("%v: dry exit %d, real exit %d", args, dry.code, real.code)
		}
		if got, want := strings.Join(withoutNext(lines(dry.stderr)), "\n"), strings.Join(withoutNext(lines(real.stderr)), "\n"); got != want {
			t.Errorf("%v: dry stderr %q, real stderr %q", args, got, want)
		}
		if strings.Contains(dry.stdout, "dry run: nothing changed") {
			t.Errorf("%v: refused dry run printed the closing line", args)
		}
	}
}

func TestVerboseStderrOnly(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := stateFixture(t, binary, "ready", "925", "true")
	for _, args := range [][]string{{"status"}, {"validate"}} {
		plain := runSplit(t, binary, root, args...)
		verbose := runSplit(t, binary, root, append(append([]string(nil), args...), "--verbose")...)
		if plain.code != verbose.code || plain.stdout != verbose.stdout {
			t.Errorf("%v: --verbose changed stdout or exit: %+v vs %+v", args, plain, verbose)
		}
		if strings.Contains(plain.stderr, "verbose: ") {
			t.Errorf("%v: verbose lines without --verbose: %q", args, plain.stderr)
		}
		if strings.Contains(verbose.stdout, "verbose: ") {
			t.Errorf("%v: verbose line on stdout: %q", args, verbose.stdout)
		}
		if !strings.Contains(verbose.stderr, "verbose: project root ") || !strings.Contains(verbose.stderr, "verbose: git -C ") {
			t.Errorf("%v: stderr lacks project root and git lines: %q", args, verbose.stderr)
		}
		for _, line := range lines(verbose.stderr) {
			if !strings.HasPrefix(line, "verbose: ") {
				t.Errorf("%v: stderr line %q does not start with verbose: ", args, line)
			}
		}
	}
	// A real mutating command: same stdout and exit code, two identical fixtures.
	a := stateFixture(t, binary, "inbox", "926", "true")
	b := stateFixture(t, binary, "inbox", "926", "true")
	plain := runSplit(t, binary, a, "promote", "TF-926")
	verbose := runSplit(t, binary, b, "promote", "TF-926", "--verbose")
	if plain.code != 0 || verbose.code != 0 || plain.stdout != verbose.stdout {
		t.Errorf("promote: %+v vs %+v", plain, verbose)
	}
	if !strings.Contains(verbose.stderr, "verbose: git commit ") || !strings.Contains(verbose.stderr, "verbose: move tasks/inbox/TF-926-example.md to tasks/ready/") {
		t.Errorf("promote --verbose stderr lacks the git commit and move lines: %q", verbose.stderr)
	}
	// A failing command keeps its exit code.
	fail := runSplit(t, binary, root, "promote", "TF-999", "--verbose")
	if fail.code != 1 {
		t.Errorf("failing promote --verbose exit = %d", fail.code)
	}
}

func TestVerboseRedactsSecrets(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	const secret = "s3cretvalue-0042"
	sub := filepath.Join(root, secret)
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"TASKFACTORY_TEST_TOKEN=" + secret}
	control := runWithEnv(t, binary, root, nil, "init", sub, "--dry-run", "--verbose")
	if control.code != 0 || !strings.Contains(control.stderr, secret) {
		t.Fatalf("control run should show the path: %+v", control)
	}
	res := runWithEnv(t, binary, root, env, "init", sub, "--dry-run", "--verbose")
	if res.code != 0 {
		t.Fatalf("%+v", res)
	}
	if strings.Contains(res.stderr, secret) {
		t.Errorf("verbose output leaks the secret: %q", res.stderr)
	}
	if !strings.Contains(res.stderr, "[redacted]") {
		t.Errorf("verbose output lacks [redacted]: %q", res.stderr)
	}
}

func TestFlagsKeepExistingUsageErrors(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	scratch := t.TempDir()
	cases := [][]string{
		{"promote"},
		{"promote", "TF-001", "TF-002"},
		{"verify"},
		{"verify", "tasks/ready/TF-001-x.md"},
		{"integrate"},
		{"check-main", "extra"},
		{"fail", "TF-001"},
		{"fail", "TF-001", "--outcome", "DONE"},
		{"requeue"},
		{"requeue", "TF-001", "--to", "active"},
		{"work", "TF-001"},
		{"work", "TF-001", "--adapter", "pi"},
		{"init", "a", "b"},
		{"status", "extra"},
	}
	for _, args := range cases {
		base := runSplit(t, binary, scratch, args...)
		if base.code != 2 {
			t.Fatalf("%v: exit %d, want 2: %+v", args, base.code, base)
		}
		for _, extra := range [][]string{{"--dry-run"}, {"--verbose"}, {"--dry-run", "--verbose"}} {
			withFlags := runSplit(t, binary, scratch, append(append([]string(nil), args...), extra...)...)
			if !reflect.DeepEqual(base, withFlags) {
				t.Errorf("%v %v: got %+v, want %+v", args, extra, withFlags, base)
			}
		}
	}
}

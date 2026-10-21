package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/testgit"
	"taskfactory/internal/ui"
)

const nextStepTaskBody = "# TF-%s: Example\n\n## Goal\n\nDo it.\n\n## Dependencies\n\nNone\n\n## Scope\n\nImplement.\n\n## Constraints\n\nKeep it small.\n\n## Success criteria\n\n### C1: It works\n\nCheck: %s\n\n## Verification\n\nRun check.\n"

type cliResult struct {
	stdout, stderr string
	code           int
}

func runSplit(t *testing.T, binary, dir string, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return cliResult{stdout.String(), stderr.String(), processExitCode(err)}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// nextStepProject returns an initialized temporary repository whose config,
// .gitignore and tasks are committed, with an inbox task TF-<id> whose single
// check is check.
func nextStepProject(t *testing.T, binary, id, check string) string {
	t.Helper()
	return testgit.CloneInto(t, "next\x00"+id+"\x00"+check, func(dir string) error {
		return buildNextStep(binary, dir, id, check)
	}, projectDir(t))
}

// buildNextStep makes the template behind nextStepProject: an initialized
// repository with config, .gitignore and the inbox task committed. It has no
// worktree and no text holding its own path.
func buildNextStep(binary, dir, id, check string) error {
	if err := testgit.Init(dir, "commit.gpgsign", "false", "user.name", "Test", "user.email", "test@example.invalid"); err != nil {
		return err
	}
	if err := testgit.Run(dir, binary, "init"); err != nil {
		return err
	}
	config := "protocol_version = 1\n\n[workers]\nmax_parallel = 4\n\n[git]\nuse_worktrees = true\nworktree_root = \".taskfactory/worktrees\"\nintegration_strategy = \"ff-only\"\n\n[integration]\nstop_on_main_failure = true\n\n[verification]\nworker = [\"true\"]\nintegration = [\"true\"]\n"
	if err := os.WriteFile(filepath.Join(dir, ".taskfactory", "config.toml"), []byte(config), 0o644); err != nil {
		return err
	}
	task := strings.Replace(strings.Replace(nextStepTaskBody, "%s", id, 1), "%s", check, 1)
	if err := os.WriteFile(filepath.Join(dir, "tasks", "inbox", "TF-"+id+"-example.md"), []byte(task), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks", "inbox", ".gitkeep"), nil, 0o644); err != nil {
		return err
	}
	if err := testgit.Git(dir, "add", "."); err != nil {
		return err
	}
	return testgit.Git(dir, "commit", "--no-gpg-sign", "-q", "-m", "initial")
}

func lines(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func countNext(s string) int {
	n := 0
	for _, line := range lines(s) {
		if strings.HasPrefix(line, "next: ") {
			n++
		}
	}
	return n
}

const wantInitBlock = "Next steps:\n" +
	"  1. Review .taskfactory/config.toml and set the verification commands.\n" +
	"  2. Commit .taskfactory/config.toml, .gitignore and tasks/ first.\n" +
	"  3. Write a task in tasks/inbox, then run: taskfactory promote <ID>\n" +
	"  4. Run: taskfactory claim <ID> --owner <name>\n" +
	"  5. Optional: install the agent skill with taskfactory skill install (check with taskfactory skill status)\n"

func TestNextStepInitBlock(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	res := runSplit(t, binary, root, "init")
	if res.code != 0 || res.stdout != wantInitBlock || res.stderr != "" {
		t.Fatalf("init: %+v, want stdout %q", res, wantInitBlock)
	}
	for _, d := range []string{".claude", ".agents"} {
		if _, err := os.Stat(filepath.Join(root, d)); err == nil {
			t.Errorf("init wrote %s", d)
		}
	}
}

func TestNextStepInitRerun(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	runSplit(t, binary, root, "init")
	res := runSplit(t, binary, root, "init")
	if res.code != 0 || res.stdout != wantInitBlock {
		t.Fatalf("second init: %+v", res)
	}
}

func TestNextStepInitWithPathNamesTarget(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	res := runSplit(t, binary, t.TempDir(), "init", root)
	if res.code != 0 || !strings.Contains(res.stdout, "2. Commit .taskfactory/config.toml, .gitignore and tasks/ first, in "+root+".\n") {
		t.Fatalf("init with path: %+v", res)
	}
}

func TestNextStepInfoCommandsOneLine(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := nextStepProject(t, binary, "901", "true")
	for _, tt := range []struct{ name, want string }{
		{"status", "next: taskfactory validate"},
		{"validate", "next: taskfactory promote <ID> or claim <ID>"},
	} {
		res := runSplit(t, binary, root, tt.name)
		if res.code != 0 || countNext(res.stdout) != 1 || !strings.HasSuffix(res.stdout, tt.want+"\n") || strings.Contains(res.stdout, "Next steps") {
			t.Errorf("%s: %+v", tt.name, res)
		}
	}
}

func TestNextStepLifecycleSuccess(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := nextStepProject(t, binary, "901", "true")
	steps := []struct {
		args []string
		want string
	}{
		{[]string{"promote", "TF-901"}, "next: taskfactory claim TF-901 --owner <name>"},
		{[]string{"claim", "TF-901", "--owner", "w"}, "next: taskfactory work TF-901 --adapter <name> --model <id>"},
		{[]string{"fail", "TF-901", "--outcome", "BLOCKED", "--reason", "x"}, "next: taskfactory requeue TF-901 --to ready"},
		{[]string{"requeue", "TF-901", "--discard-claim"}, "next: taskfactory claim TF-901 --owner <name>"},
		{[]string{"claim", "TF-901", "--owner", "w"}, "next: taskfactory work TF-901 --adapter <name> --model <id>"},
		{[]string{"fail", "TF-901", "--outcome", "BLOCKED", "--reason", "x"}, "next: taskfactory requeue TF-901 --to ready"},
		{[]string{"requeue", "TF-901", "--to", "inbox", "--discard-claim"}, "next: taskfactory promote TF-901"},
	}
	for _, step := range steps {
		res := runSplit(t, binary, root, step.args...)
		if res.code != 0 || !strings.HasSuffix(res.stdout, step.want+"\n") || countNext(res.stdout) != 1 || res.stderr != "" {
			t.Fatalf("%v: %+v, want last line %q", step.args, res, step.want)
		}
	}
}

// passingLifecycle runs promote, claim, a commit in the worktree, verify and
// integrate, returning the output of verify and integrate.
func passingLifecycle(t *testing.T, binary string) (root string, verify, integrate cliResult) {
	t.Helper()
	root = nextStepProject(t, binary, "902", "true")
	for _, args := range [][]string{{"promote", "TF-902"}, {"claim", "TF-902", "--owner", "w"}} {
		if res := runSplit(t, binary, root, args...); res.code != 0 {
			t.Fatalf("%v: %+v", args, res)
		}
	}
	worktree := filepath.Join(root, ".taskfactory", "worktrees", "TF-902")
	if err := os.WriteFile(filepath.Join(worktree, "feature.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, worktree, "add", "feature.txt")
	mustGit(t, worktree, "commit", "--no-gpg-sign", "-m", "feature")
	verify = runSplit(t, binary, root, "verify", "TF-902")
	integrate = runSplit(t, binary, root, "integrate", "TF-902")
	return root, verify, integrate
}

func TestNextStepVerifyPass(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	_, verify, _ := passingLifecycle(t, binary)
	want := "verify TF-902: PASS (2 checks)\nnext: taskfactory integrate TF-902\n"
	if verify.code != 0 || verify.stdout != want || verify.stderr != "" {
		t.Fatalf("verify: %+v, want stdout %q", verify, want)
	}
}

func TestNextStepVerifyFail(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := nextStepProject(t, binary, "903", "false")
	runSplit(t, binary, root, "promote", "TF-903")
	runSplit(t, binary, root, "claim", "TF-903", "--owner", "w")
	res := runSplit(t, binary, root, "verify", "TF-903")
	if res.code != 1 || countNext(res.stdout) != 0 || !strings.HasPrefix(res.stdout, "verify TF-903: FAIL") {
		t.Fatalf("verify stdout/exit: %+v", res)
	}
	errLines := lines(res.stderr)
	if last := errLines[len(errLines)-1]; last != "next: fix the checks, rerun taskfactory verify TF-903" || !strings.HasPrefix(errLines[0], "taskfactory verify: ") {
		t.Fatalf("verify stderr = %q", res.stderr)
	}
}

func TestNextStepIntegrateAndCheckMain(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root, _, integrate := passingLifecycle(t, binary)
	if integrate.code != 0 || integrate.stdout != "integrated TF-902\nnext: taskfactory status\n" {
		t.Fatalf("integrate: %+v", integrate)
	}
	res := runSplit(t, binary, root, "check-main")
	if res.code != 0 || res.stdout != "check-main: ok\nnext: taskfactory integrate <ID>\n" {
		t.Fatalf("check-main: %+v", res)
	}
	bad := runSplit(t, binary, root, "integrate", "TF-999")
	if bad.code != 1 || bad.stdout != "" || !strings.HasSuffix(bad.stderr, "\nnext: rerun taskfactory verify TF-999, then integrate TF-999\n") {
		t.Fatalf("integrate failure: %+v", bad)
	}
	bad = runSplit(t, binary, t.TempDir(), "check-main")
	if bad.code != 1 || bad.stdout != "" || !strings.HasSuffix(bad.stderr, "\nnext: fix main, rerun taskfactory check-main\n") {
		t.Fatalf("check-main failure: %+v", bad)
	}
}

func TestNextStepFailureHintsOnStderr(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := nextStepProject(t, binary, "904", "true")
	status := "\nnext: taskfactory status shows the task state\n"
	for _, args := range [][]string{
		{"fail", "TF-999", "--outcome", "FAILED", "--reason", "x"},
		{"requeue", "TF-999"},
		{"claim", "TF-999", "--owner", "w"},
	} {
		res := runSplit(t, binary, root, args...)
		if res.code != 1 || res.stdout != "" || !strings.HasSuffix(res.stderr, status) {
			t.Errorf("%v: %+v", args, res)
		}
	}
	res := runSplit(t, binary, root, "promote", "TF-999")
	if res.code != 1 || res.stdout != "" || !strings.HasSuffix(res.stderr, "\nnext: fix tasks/inbox, rerun taskfactory promote TF-999\n") {
		t.Errorf("promote: %+v", res)
	}
	res = runSplit(t, binary, t.TempDir(), "status")
	if res.code != 1 || !strings.HasSuffix(res.stderr, "\nnext: run taskfactory init at the repo root\n") {
		t.Errorf("status: %+v", res)
	}
	res = runSplit(t, binary, t.TempDir(), "validate")
	if res.code != 1 || !strings.HasSuffix(res.stderr, "\nnext: fix the files, rerun taskfactory validate\n") {
		t.Errorf("validate: %+v", res)
	}
}

func TestNextStepUsageErrorsHaveNoHint(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := nextStepProject(t, binary, "905", "true")
	for _, args := range [][]string{
		{"verify"},
		{"verify", "a", "b"},
		{"integrate"},
		{"promote"},
		{"check-main", "extra"},
		{"fail"},
		{"fail", "TF-001"},
		{"requeue"},
		{"requeue", "TF-001", "--to", "active"},
		{"validate", "/etc/passwd"},
		{"init", "-x"},
		{"init", "a", "b"},
		{"bogus"},
	} {
		res := runSplit(t, binary, root, args...)
		if res.code != 2 || strings.Contains(res.stdout+res.stderr, "next:") || strings.Contains(res.stdout+res.stderr, "Next steps") {
			t.Errorf("%v: %+v", args, res)
		}
	}
}

func TestNextStepMachineOutputUnchanged(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := nextStepProject(t, binary, "906", "true")
	status := runSplit(t, binary, root, "status")
	wantStatus := "inbox: 1\nready: 0\nactive: 0\nfailed: 0\narchive: 0\n"
	if status.code != 0 || !strings.HasPrefix(status.stdout, wantStatus) {
		t.Errorf("status: %+v", status)
	}
	for _, tt := range []struct {
		args  []string
		first string
	}{
		{[]string{"promote", "TF-906"}, "promoted TF-906 to tasks/ready/TF-906-example.md\n"},
		{[]string{"claim", "TF-906", "--owner", "w"}, "claimed TF-906 for w\n"},
	} {
		res := runSplit(t, binary, root, tt.args...)
		if res.code != 0 || !strings.HasPrefix(res.stdout, tt.first) {
			t.Errorf("%v: %+v, want first line %q", tt.args, res, tt.first)
		}
	}
	_, verify, integrate := passingLifecycle(t, binary)
	if verify.code != 0 || !strings.HasPrefix(verify.stdout, "verify TF-902: PASS (2 checks)\n") {
		t.Errorf("verify: %+v", verify)
	}
	if integrate.code != 0 || !strings.HasPrefix(integrate.stdout, "integrated TF-902\n") {
		t.Errorf("integrate: %+v", integrate)
	}
}

func TestNextStepHelpLinePerCommand(t *testing.T) {
	t.Parallel()
	for name, help := range commandHelps {
		head, _, ok := strings.Cut(help, "\nFlags:\n")
		if !ok {
			t.Errorf("%s help has no Flags section", name)
			continue
		}
		found := 0
		for _, line := range lines(head) {
			if strings.HasPrefix(line, "Next step:") {
				found++
			}
		}
		if found != 1 {
			t.Errorf("%s help has %d Next step lines, want 1", name, found)
		}
	}
	if strings.Contains(usage(ui.Painter{}), "Next step") {
		t.Error("top-level usage must not carry per-command hints")
	}
}

func TestNextStepTableIsSingleSource(t *testing.T) {
	t.Parallel()
	for _, command := range commandSummaries {
		if command.name == "init" {
			continue
		}
		key := command.name
		if key == "skill" {
			key = "skill-install" // skill has the two entries skill-install and skill-status
		}
		if _, ok := nextSteps[key]; !ok {
			t.Errorf("no next-step table entry for %s", command.name)
		}
	}
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(main), `"next: `) || strings.Contains(string(main), `next: taskfactory`) {
		t.Error("main.go writes a hint string of its own")
	}
	workSource, err := os.ReadFile(filepath.Join("..", "..", "internal", "work", "work.go"))
	if err != nil {
		t.Fatal(err)
	}
	want := `"next: ` + strings.ReplaceAll(nextSteps["work"].success, "<ID>", "%s") + `\n"`
	if !strings.Contains(string(workSource), want) {
		t.Errorf("internal/work does not print %s", want)
	}
	if got := nextOnSuccess(ui.Painter{}, "work", "TF-1"); got != "next: taskfactory verify TF-1" {
		t.Errorf("work hint = %q", got)
	}
}

// Serial: sets process-global environment or working directory, which is unsafe in a parallel test.
func TestNextStepPlainWithoutColor(t *testing.T) {
	plain := ui.Painter{}
	if got := nextOnSuccess(plain, "verify", "TF-7"); got != "next: taskfactory integrate TF-7" {
		t.Errorf("plain success = %q", got)
	}
	if got := nextOnFailure(plain, "verify", "TF-7"); got != "next: fix the checks, rerun taskfactory verify TF-7" {
		t.Errorf("plain failure = %q", got)
	}
	if got := nextOnFailure(plain, "init", ""); got != "" {
		t.Errorf("init failure hint = %q, want none", got)
	}
	if got := nextOnSuccess(plain, "init", "") + "\n"; got != wantInitBlock {
		t.Errorf("init block = %q", got)
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm")
	t.Setenv("FORCE_COLOR", "1")
	colored := ui.For(os.Stdout)
	got := nextOnSuccess(colored, "status", "")
	if got != "\x1b[2mnext:\x1b[0m taskfactory validate" {
		t.Errorf("colored hint = %q", got)
	}
}

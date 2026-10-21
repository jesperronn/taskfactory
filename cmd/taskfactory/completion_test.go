package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"taskfactory/internal/ui"
)

// completionFixture is a git repository with task files in every state and
// verification evidence for one active task.
func completionFixture(t *testing.T) string {
	t.Helper()
	root := initGitProject(t)
	files := map[string][]string{
		"inbox":  {"TF-010-inbox-one.md"},
		"ready":  {"TF-002-ready-b.md", "TF-001-ready-a.md", "NOTES.md", "TF-12-short.md"},
		"active": {"TF-003-active-a.md", "TF-004-active-b.md"},
		"failed": {"TF-005-failed.md"},
	}
	for state, names := range files {
		dir := filepath.Join(root, "tasks", state)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	evidence := filepath.Join(root, ".taskfactory", "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "TF-004.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCompletionScriptsListEveryCommand(t *testing.T) {
	binary := buildCLI(t)
	for _, shell := range []string{"bash", "zsh"} {
		res := runSplit(t, binary, t.TempDir(), "completion", shell)
		if res.code != 0 {
			t.Fatalf("completion %s: %+v", shell, res)
		}
		for _, c := range commandSummaries {
			if !regexp.MustCompile(`\b` + regexp.QuoteMeta(c.name) + `\b`).MatchString(res.stdout) {
				t.Errorf("%s script does not mention %q", shell, c.name)
			}
		}
		if !strings.Contains(res.stdout, "--version") {
			t.Errorf("%s script lacks the top-level flags", shell)
		}
	}
}

func TestCompletionTableMatchesCommands(t *testing.T) {
	if len(completionSpecs) != len(commandSummaries) {
		t.Errorf("completionSpecs has %d entries, commandSummaries %d", len(completionSpecs), len(commandSummaries))
	}
	seen := map[string]bool{}
	for _, s := range completionSpecs {
		if seen[s.Command] {
			t.Errorf("duplicate completion entry %q", s.Command)
		}
		seen[s.Command] = true
	}
	for _, c := range commandSummaries {
		spec, ok := findCompletionSpec(c.name)
		if !ok {
			t.Errorf("command %q has no completionSpecs entry", c.name)
			continue
		}
		help := commandHelps[c.name]
		for _, f := range spec.Flags {
			if !strings.Contains(help, f.Name) {
				t.Errorf("%s: table flag %s is not in the command help", c.name, f.Name)
			}
		}
		// The flags the contract lists for each command must stay a subset of
		// the command's real flags; the common flags are checked below.
	}
	for _, name := range []string{"status", "claim"} {
		spec, _ := findCompletionSpec(name)
		var got []string
		for _, f := range spec.allFlags() {
			got = append(got, f.Name)
		}
		if name == "status" && strings.Join(got, " ") != "--verbose --dry-run --help" {
			t.Errorf("status flags = %v", got)
		}
	}
	spec, _ := findCompletionSpec("completion")
	for _, f := range spec.allFlags() {
		if f.Name == "--verbose" || f.Name == "--dry-run" {
			t.Errorf("completion must not offer %s", f.Name)
		}
	}
}

func TestCompleteHelperIDsPerCommand(t *testing.T) {
	binary := buildCLI(t)
	root := completionFixture(t)
	want := map[string]string{
		"promote":   "TF-010",
		"claim":     "TF-001\nTF-002",
		"work":      "TF-003\nTF-004",
		"verify":    "TF-003\nTF-004",
		"fail":      "TF-003\nTF-004",
		"integrate": "TF-004",
		"requeue":   "TF-005",
	}
	for command, ids := range want {
		res := runSplit(t, binary, root, "__complete", "ids", command)
		if res.code != 0 || res.stderr != "" || strings.TrimSpace(res.stdout) != ids {
			t.Errorf("__complete ids %s = %+v, want %q", command, res, ids)
		}
	}
	for _, command := range []string{"status", "init", "nonexistent"} {
		if res := runSplit(t, binary, root, "__complete", "ids", command); res.code != 0 || res.stdout != "" || res.stderr != "" {
			t.Errorf("__complete ids %s = %+v, want nothing", command, res)
		}
	}
	if res := runSplit(t, binary, root, "__complete", "bogus", "claim"); res.code != 0 || res.stdout != "" || res.stderr != "" {
		t.Errorf("unknown kind = %+v", res)
	}
	if res := runSplit(t, binary, root, "__complete"); res.code != 0 || res.stdout != "" || res.stderr != "" {
		t.Errorf("no arguments = %+v", res)
	}
	// A missing state directory yields nothing.
	if err := os.RemoveAll(filepath.Join(root, "tasks", "failed")); err != nil {
		t.Fatal(err)
	}
	if res := runSplit(t, binary, root, "__complete", "ids", "requeue"); res.code != 0 || res.stdout != "" || res.stderr != "" {
		t.Errorf("missing directory = %+v", res)
	}
}

func TestCompleteHelperOutsideRepo(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	for _, command := range []string{"claim", "integrate", "promote"} {
		res := runSplit(t, binary, dir, "__complete", "ids", command, "TF-")
		if res.code != 0 || res.stdout != "" || res.stderr != "" {
			t.Errorf("outside a repository, %s = %+v", command, res)
		}
	}
}

func TestCompleteHelperSubdirAndPrefix(t *testing.T) {
	binary := buildCLI(t)
	root := completionFixture(t)
	sub := filepath.Join(root, "tasks", "ready")
	res := runSplit(t, binary, sub, "__complete", "ids", "claim")
	if res.code != 0 || strings.TrimSpace(res.stdout) != "TF-001\nTF-002" {
		t.Errorf("from subdirectory = %+v", res)
	}
	res = runSplit(t, binary, sub, "__complete", "ids", "claim", "TF-002")
	if res.code != 0 || strings.TrimSpace(res.stdout) != "TF-002" {
		t.Errorf("prefix TF-002 = %+v", res)
	}
	res = runSplit(t, binary, sub, "__complete", "ids", "claim", "TF-9")
	if res.code != 0 || res.stdout != "" {
		t.Errorf("prefix TF-9 = %+v", res)
	}
}

func TestCompletionUsageErrors(t *testing.T) {
	binary := buildCLI(t)
	for _, args := range [][]string{{"completion"}, {"completion", "fish"}, {"completion", "bash", "extra"}, {"completion", "bash", "--verbose"}} {
		res := runSplit(t, binary, t.TempDir(), args...)
		if res.code != 2 || res.stdout != "" || !strings.HasPrefix(res.stderr, "taskfactory completion: ") {
			t.Errorf("%v = %+v, want exit 2 with a completion error on stderr", args, res)
		}
	}
}

// bashComplete sources the real bash script, sets COMP_WORDS and COMP_CWORD,
// calls _taskfactory and returns COMPREPLY.
func bashComplete(t *testing.T, binary, dir string, words ...string) []string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "completion.bash")
	out, err := exec.Command(binary, "completion", "bash").Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, out, 0o644); err != nil {
		t.Fatal(err)
	}
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = "'" + w + "'"
	}
	driver := `source "$1"; COMP_WORDS=(` + strings.Join(quoted, " ") + `); COMP_CWORD=$((${#COMP_WORDS[@]}-1)); _taskfactory; printf '%s\n' "${COMPREPLY[@]}"`
	cmd := exec.Command("bash", "-c", driver, "bash", script)
	cmd.Dir = dir
	bin := t.TempDir()
	for _, name := range []string{"taskfactory", "tf"} {
		if err := os.Symlink(binary, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := cmd.Output()
	if err != nil {
		t.Fatalf("bash driver for %v: %v", words, err)
	}
	text := strings.TrimSpace(string(got))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func TestBashCompletion(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	binary := buildCLI(t)
	script, err := exec.Command(binary, "completion", "bash").Output()
	if err != nil {
		t.Fatal(err)
	}
	syntax := exec.Command("bash", "-n")
	syntax.Stdin = strings.NewReader(string(script))
	if out, err := syntax.CombinedOutput(); err != nil {
		t.Fatalf("bash -n: %v\n%s", err, out)
	}
	root := completionFixture(t)
	cases := []struct {
		words []string
		want  string
	}{
		{[]string{"taskfactory", "pro"}, "promote"},
		{[]string{"taskfactory", "claim", "TF-"}, "TF-001 TF-002"},
		{[]string{"taskfactory", "claim", "TF-001", ""}, ""},
		{[]string{"taskfactory", "integrate", ""}, "TF-004"},
		{[]string{"taskfactory", "work", "TF-001", "--adapter", ""}, "omp pi claude"},
		{[]string{"taskfactory", "fail", "TF-001", "--outcome", ""}, "FAILED BLOCKED"},
		{[]string{"taskfactory", "requeue", "TF-001", "--to", ""}, "ready inbox"},
		{[]string{"tf", "status", "--"}, "--verbose --dry-run --help"},
		{[]string{"tf", "status", "--verbose", "--"}, "--dry-run --help"},
		{[]string{"tf", "claim", "--owner", ""}, ""},
		{[]string{"tf", "claim", "--owner", "bob", ""}, "TF-001 TF-002"},
		{[]string{"tf", "claim", "--", "--"}, ""},
		{[]string{"tf", "completion", ""}, "bash zsh"},
		{[]string{"tf", "completion", "--"}, "--help"},
		{[]string{"tf", "--"}, "--help --version"},
		{[]string{"tf", "nonesuch", ""}, ""},
	}
	for _, c := range cases {
		got := strings.Join(bashComplete(t, binary, root, c.words...), " ")
		if got != c.want {
			t.Errorf("%q: COMPREPLY = %q, want %q", c.words, got, c.want)
		}
	}
	if got := bashComplete(t, binary, root, "taskfactory", ""); len(got) != len(commandSummaries) {
		t.Errorf("top level offers %v, want %d commands", got, len(commandSummaries))
	}
}

func TestZshCompletionSyntax(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh not available; skipping the zsh syntax check")
	}
	binary := buildCLI(t)
	script, err := exec.Command(binary, "completion", "zsh").Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(zsh, "-n")
	cmd.Stdin = strings.NewReader(string(script))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("zsh -n: %v\n%s", err, out)
	}
	if !strings.HasPrefix(string(script), "#compdef taskfactory tf\n") {
		t.Errorf("zsh script does not start with #compdef")
	}
}

func TestCompletionRegistersTfAlias(t *testing.T) {
	binary := buildCLI(t)
	for shell, line := range map[string]string{
		"bash": "complete -F _taskfactory taskfactory tf",
		"zsh":  "compdef _taskfactory taskfactory tf",
	} {
		res := runSplit(t, binary, t.TempDir(), "completion", shell)
		if !strings.Contains(res.stdout, line+"\n") {
			t.Errorf("%s script lacks %q", shell, line)
		}
	}
}

func TestCompletionHelpAndHint(t *testing.T) {
	binary := buildCLI(t)
	scratch := t.TempDir()
	res := runSplit(t, binary, scratch, "completion", "--help")
	if res.code != 0 || res.stdout != completionHelp || !strings.HasPrefix(res.stdout, "Usage: taskfactory completion <bash|zsh>\n") {
		t.Errorf("completion --help = %+v", res)
	}
	top := runSplit(t, binary, scratch, "--help")
	if !strings.Contains(top.stdout, "completion <bash|zsh>") {
		t.Errorf("top-level usage lacks completion: %s", top.stdout)
	}
	if strings.Contains(top.stdout, "__complete") || strings.Contains(res.stdout, "__complete") {
		t.Error("the hidden helper must not appear in help")
	}
	h := nextSteps["completion"]
	if h.success != "add it to your shell profile, see README" || h.failure != "taskfactory completion bash or taskfactory completion zsh" {
		t.Errorf("hint table entry = %+v", h)
	}
	if got := nextOnSuccess(ui.Painter{}, "completion", ""); got != "next: add it to your shell profile, see README" {
		t.Errorf("success hint = %q", got)
	}
	// Piped stdout carries the script only, with no hint anywhere.
	piped := runSplit(t, binary, scratch, "completion", "bash")
	if piped.stderr != "" || countNext(piped.stdout) != 0 {
		t.Errorf("piped completion printed a hint: %+v", piped.stderr)
	}
	// The terminal path prints the hint on stderr only.
	var out, errBuf strings.Builder
	if code := completionCommand([]string{"bash"}, &out, &errBuf, true); code != 0 || !strings.Contains(errBuf.String(), "next: add it to your shell profile") || strings.Contains(out.String(), "next:") {
		t.Errorf("terminal hint: code=%d stderr=%q", code, errBuf.String())
	}
	// __complete prints no hint.
	if r := runSplit(t, binary, scratch, "__complete", "ids", "claim"); r.stderr != "" || r.stdout != "" {
		t.Errorf("__complete = %+v", r)
	}
}

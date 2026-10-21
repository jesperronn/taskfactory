package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNewWritesValidSkeletonWithNextFreeID(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	mustGit(t, root, "config", "user.name", "Test")
	mustGit(t, root, "config", "user.email", "test@example.invalid")
	if res := runSplit(t, binary, root, "init"); res.code != 0 {
		t.Fatalf("init: %+v", res)
	}
	// The highest ID is in the archive, so the next free ID skips past it.
	archived := "# TF-041: Old\n"
	if err := os.WriteFile(filepath.Join(root, "tasks", "archive", "TF-041-old.md"), []byte(archived), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tasks", "ready", "TF-007-ready.md"), []byte(nextStepTaskBody7), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "add", ".")
	mustGit(t, root, "commit", "--no-gpg-sign", "-q", "-m", "fixture")
	head := runGitOut(t, root, "rev-parse", "HEAD")

	res := runSplit(t, binary, root, "new", "  Add the Über   Widget (v2)!  ")
	if res.code != 0 {
		t.Fatalf("new: %+v", res)
	}
	const rel = "tasks/inbox/TF-042-add-the-ber-widget-v2.md"
	if !strings.HasPrefix(res.stdout, "created "+rel+"\n") {
		t.Errorf("stdout = %q", res.stdout)
	}
	if !strings.Contains(res.stdout, "next: edit the file, then taskfactory validate and taskfactory promote TF-042") {
		t.Errorf("stdout lacks the validate/promote hint: %q", res.stdout)
	}
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "# TF-042: Add the Über   Widget (v2)!\n") {
		t.Errorf("heading wrong: %q", text)
	}
	var headings []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			headings = append(headings, line)
		}
	}
	want := []string{"## Goal", "## Dependencies", "## Scope", "## Constraints", "## Success criteria", "## Verification"}
	if !reflect.DeepEqual(headings, want) {
		t.Errorf("headings = %v, want %v", headings, want)
	}
	if !strings.Contains(text, "### C1: TODO") || !strings.Contains(text, "\nCheck: TODO") || !strings.Contains(text, "## Dependencies\n\nNone\n") {
		t.Errorf("criterion, Check or Dependencies wrong:\n%s", text)
	}
	if v := runSplit(t, binary, root, "validate", rel); v.code != 0 {
		t.Errorf("validate of the skeleton: %+v", v)
	}
	if v := runSplit(t, binary, root, "validate"); v.code != 0 {
		t.Errorf("validate of the tree: %+v", v)
	}
	if got := runGitOut(t, root, "rev-parse", "HEAD"); got != head {
		t.Errorf("new committed something: HEAD %s, was %s", got, head)
	}
	if status := runGitOut(t, root, "status", "--short", "--untracked-files=all"); strings.TrimSpace(status) != "?? "+rel {
		t.Errorf("new wrote more than the one file:\n%s", status)
	}
	// Promote refuses the skeleton while TODO markers remain.
	if p := runSplit(t, binary, root, "promote", "TF-042"); p.code != 1 || !strings.Contains(p.stderr, "TODO") {
		t.Errorf("promote of the skeleton: %+v", p)
	}
	// A second run picks the next ID and never overwrites.
	if r := runSplit(t, binary, root, "new", "Second"); r.code != 0 || !strings.Contains(r.stdout, "TF-043-second.md") {
		t.Errorf("second new: %+v", r)
	}
}

func TestNewRefusesBadTitles(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := nextStepProject(t, binary, "001", "true")
	before := snapshot(t, root)
	for _, args := range [][]string{
		{"new"}, {"new", ""}, {"new", "   "}, {"new", "!!! ???"}, {"new", "ÄÖ"}, {"new", "a\nb"}, {"new", "one", "two"}, {"new", "-x"},
	} {
		res := runSplit(t, binary, root, args...)
		if res.code != 2 || !strings.Contains(res.stderr, "taskfactory new") || res.stdout != "" {
			t.Errorf("%q: %+v, want exit 2 with an error and no stdout", args, res)
		}
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("a refused new changed the project")
	}
}

func TestNewDryRunAndVerbose(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := nextStepProject(t, binary, "005", "true")
	before := snapshot(t, root)
	res := runSplit(t, binary, root, "new", "Plan it", "--dry-run", "--verbose")
	if res.code != 0 {
		t.Fatalf("dry run: %+v", res)
	}
	if res.stdout != "would: write tasks/inbox/TF-006-plan-it.md\ndry run: nothing changed\n" {
		t.Errorf("dry-run stdout = %q", res.stdout)
	}
	if !strings.Contains(res.stderr, "verbose: next free task ID TF-006") {
		t.Errorf("verbose stderr = %q", res.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("dry run changed the project")
	}
	real := runSplit(t, binary, root, "new", "Plan it", "--verbose")
	if real.code != 0 || !strings.Contains(real.stderr, "verbose: write tasks/inbox/TF-006-plan-it.md") {
		t.Errorf("verbose run: %+v", real)
	}
}

func TestNewHonorsTasksDir(t *testing.T) {
	t.Parallel()
	binary := buildCLI(t)
	root := initGitProject(t)
	mustGit(t, root, "config", "user.name", "Test")
	mustGit(t, root, "config", "user.email", "test@example.invalid")
	if res := runSplit(t, binary, root, "init"); res.code != 0 {
		t.Fatalf("init: %+v", res)
	}
	cfgPath := filepath.Join(root, ".taskfactory", "config.toml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, append([]byte("tasks_dir = \".taskfactory/tasks\"\n"), cfg...), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		if err := os.MkdirAll(filepath.Join(root, ".taskfactory", "tasks", state), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".taskfactory", "tasks", "archive", "TF-010-old.md"), []byte("# TF-010: Old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := runSplit(t, binary, root, "new", "Configured dir")
	if res.code != 0 || !strings.HasPrefix(res.stdout, "created .taskfactory/tasks/inbox/TF-011-configured-dir.md\n") {
		t.Fatalf("new with tasks_dir: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "inbox", "TF-011-configured-dir.md")); err == nil {
		t.Error("new wrote under the default tasks/ directory")
	}
}

var nextStepTaskBody7 = strings.Replace(strings.Replace(nextStepTaskBody, "%s", "007", 1), "%s", "true", 1)

func runGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runCLIGit(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func runCLIGit(dir string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
}

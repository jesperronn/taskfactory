package promote

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func validTask(id, slug string) string {
	return "# " + id + ": Example " + slug + "\n\n" +
		"## Goal\n\nMake the example change.\n\n" +
		"## Dependencies\n\nNone\n\n" +
		"## Scope\n\nTouch only the example file.\n\n" +
		"## Constraints\n\nUse only the standard library.\n\n" +
		"## Success criteria\n\n### C1: Example passes\n\nCheck: go test ./...\n\n" +
		"## Verification\n\nRun the check from the repository root and report its exit code.\n"
}

func invalidTask(id string) string {
	return "# " + id + ": Missing criteria\n\n## Goal\n\nDo the work.\n\n" +
		"## Dependencies\n\nNone\n\n## Scope\n\nImplement it.\n\n" +
		"## Constraints\n\nKeep it small.\n\n## Success criteria\n\n" +
		"## Verification\n\nRun checks.\n"
}

// newRepo creates a temporary repository on main with one commit. Global and
// system Git configuration are disabled so only the fixture identity applies.
func newRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	root := t.TempDir()
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.name", "Test Worker")
	runGit(t, root, "config", "user.email", "worker@example.invalid")
	runGit(t, root, "config", "commit.gpgsign", "false")
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		dir := filepath.Join(root, "tasks", state)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, ".gitkeep"), "")
	}
	writeFile(t, filepath.Join(root, "README.md"), "fixture\n")
	commitAll(t, root, "fixture")
	return root
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func commitAll(t *testing.T, root, message string) {
	t.Helper()
	runGit(t, root, "add", "--all")
	runGit(t, root, "commit", "-q", "--allow-empty", "--no-gpg-sign", "-m", message)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTask(t *testing.T, root, state, name, content string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "tasks", state, name), content)
}

func head(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
}

// snapshotTasks returns the content of every file below tasks/, keyed by path.
func snapshotTasks(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(filepath.Join(root, "tasks"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			files[filepath.ToSlash(rel)] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func sameFiles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func lines(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.Trim(s, "\n"), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestPromotesValidInboxTask(t *testing.T) {
	const name = "TF-010-valid-example.md"
	content := validTask("TF-010", "valid-example")

	t.Run("tracked proposal", func(t *testing.T) {
		root := newRepo(t)
		writeTask(t, root, "inbox", name, content)
		commitAll(t, root, "add proposal")

		rel, err := Promote(root, "TF-010")
		if err != nil {
			t.Fatalf("Promote: %v", err)
		}
		if rel != "tasks/ready/"+name {
			t.Fatalf("ready path = %q", rel)
		}
		if _, err := os.Stat(filepath.Join(root, "tasks", "inbox", name)); !os.IsNotExist(err) {
			t.Fatalf("inbox file still present: %v", err)
		}
		got, err := os.ReadFile(filepath.Join(root, "tasks", "ready", name))
		if err != nil || string(got) != content {
			t.Fatalf("ready contents changed or missing: %v", err)
		}
		if subject := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); subject != "docs: promote TF-010 to ready" {
			t.Fatalf("commit subject = %q", subject)
		}
		changed := lines(runGit(t, root, "show", "--no-renames", "--name-only", "--format=", "HEAD"))
		want := []string{"tasks/inbox/" + name, "tasks/ready/" + name}
		sort.Strings(want)
		if strings.Join(changed, ",") != strings.Join(want, ",") {
			t.Fatalf("commit paths = %v, want %v", changed, want)
		}
		if status := runGit(t, root, "status", "--porcelain"); status != "" {
			t.Fatalf("tree not clean after promotion:\n%s", status)
		}
	})

	t.Run("untracked proposal", func(t *testing.T) {
		root := newRepo(t)
		writeTask(t, root, "inbox", name, content)

		if _, err := Promote(root, "TF-010"); err != nil {
			t.Fatalf("Promote: %v", err)
		}
		changed := lines(runGit(t, root, "show", "--no-renames", "--name-only", "--format=", "HEAD"))
		if len(changed) != 1 || changed[0] != "tasks/ready/"+name {
			t.Fatalf("commit paths = %v, want only the ready path", changed)
		}
	})
}

func TestRefusesInvalidContractWrongStateAndUnknownID(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		setup func(t *testing.T, root string)
	}{
		{"invalid contract", "TF-020", func(t *testing.T, root string) {
			writeTask(t, root, "inbox", "TF-020-missing-criteria.md", invalidTask("TF-020"))
		}},
		{"unresolved dependency", "TF-021", func(t *testing.T, root string) {
			task := strings.Replace(validTask("TF-021", "dep"), "## Dependencies\n\nNone", "## Dependencies\n\n- TF-500", 1)
			writeTask(t, root, "inbox", "TF-021-dep.md", task)
		}},
		{"already ready", "TF-022", func(t *testing.T, root string) {
			writeTask(t, root, "ready", "TF-022-ready.md", validTask("TF-022", "ready"))
		}},
		{"already archived", "TF-023", func(t *testing.T, root string) {
			writeTask(t, root, "archive", "TF-023-old.md", validTask("TF-023", "old"))
		}},
		{"ambiguous inbox", "TF-024", func(t *testing.T, root string) {
			writeTask(t, root, "inbox", "TF-024-a.md", validTask("TF-024", "a"))
			writeTask(t, root, "inbox", "TF-024-b.md", validTask("TF-024", "b"))
		}},
		{"unknown ID", "TF-099", func(t *testing.T, root string) {}},
		{"unrelated tree error", "TF-025", func(t *testing.T, root string) {
			writeTask(t, root, "inbox", "TF-025-valid.md", validTask("TF-025", "valid"))
			writeTask(t, root, "failed", "TF-900-broken.md", validTask("TF-901", "broken"))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newRepo(t)
			tt.setup(t, root)
			commitAll(t, root, "setup")
			before := snapshotTasks(t, root)
			beforeHead := head(t, root)

			if _, err := Promote(root, tt.id); err == nil {
				t.Fatalf("Promote(%s) succeeded, want refusal", tt.id)
			}
			if !sameFiles(before, snapshotTasks(t, root)) {
				t.Fatalf("task files changed after refusal")
			}
			if head(t, root) != beforeHead {
				t.Fatalf("HEAD moved after refusal")
			}
			if status := runGit(t, root, "status", "--porcelain"); status != "" {
				t.Fatalf("index or tree dirty after refusal:\n%s", status)
			}
		})
	}
}

func TestRefusesMalformedIDAsUsageError(t *testing.T) {
	root := newRepo(t)
	_, err := Promote(root, "TF-1")
	var usage UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("Promote(TF-1) error = %v, want UsageError", err)
	}
}

func TestRefusesCommitFailureAndRestoresFile(t *testing.T) {
	root := newRepo(t)
	name := "TF-030-hook.md"
	content := validTask("TF-030", "hook")
	writeTask(t, root, "inbox", name, content)
	commitAll(t, root, "add proposal")
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	beforeHead := head(t, root)

	if _, err := Promote(root, "TF-030"); err == nil {
		t.Fatal("Promote succeeded despite failing pre-commit hook")
	}
	got, err := os.ReadFile(filepath.Join(root, "tasks", "inbox", name))
	if err != nil || string(got) != content {
		t.Fatalf("inbox file not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "ready", name)); !os.IsNotExist(err) {
		t.Fatalf("ready file left behind: %v", err)
	}
	if head(t, root) != beforeHead {
		t.Fatal("HEAD moved after failed commit")
	}
	if staged := runGit(t, root, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("index still has staged paths:\n%s", staged)
	}
}

func TestIsolationRefusesStagedIndexAndCommitsOnlyTaskPaths(t *testing.T) {
	t.Run("staged unrelated path refused", func(t *testing.T) {
		root := newRepo(t)
		writeTask(t, root, "inbox", "TF-030-staged.md", validTask("TF-030", "staged"))
		commitAll(t, root, "add proposal")
		writeFile(t, filepath.Join(root, "other.txt"), "staged\n")
		runGit(t, root, "add", "--", "other.txt")
		before := snapshotTasks(t, root)
		beforeHead := head(t, root)

		if _, err := Promote(root, "TF-030"); err == nil {
			t.Fatal("Promote succeeded with a staged unrelated path")
		}
		if !sameFiles(before, snapshotTasks(t, root)) || head(t, root) != beforeHead {
			t.Fatal("refusal changed task files or HEAD")
		}
		if staged := lines(runGit(t, root, "diff", "--cached", "--name-only")); strings.Join(staged, ",") != "other.txt" {
			t.Fatalf("staged paths = %v, want only other.txt", staged)
		}
	})

	t.Run("dirty unrelated files are left alone", func(t *testing.T) {
		root := newRepo(t)
		writeTask(t, root, "inbox", "TF-031-dirty.md", validTask("TF-031", "dirty"))
		commitAll(t, root, "add proposal")
		writeFile(t, filepath.Join(root, "README.md"), "changed\n")
		writeFile(t, filepath.Join(root, "scratch.txt"), "untracked\n")

		if _, err := Promote(root, "TF-031"); err != nil {
			t.Fatalf("Promote: %v", err)
		}
		changed := lines(runGit(t, root, "show", "--no-renames", "--name-only", "--format=", "HEAD"))
		if strings.Join(changed, ",") != "tasks/inbox/TF-031-dirty.md,tasks/ready/TF-031-dirty.md" {
			t.Fatalf("commit paths = %v", changed)
		}
		status := lines(runGit(t, root, "status", "--porcelain"))
		if strings.Join(status, "|") != " M README.md|?? scratch.txt" {
			t.Fatalf("status after promotion = %v", status)
		}
	})
}

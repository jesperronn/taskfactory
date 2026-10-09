package fail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertTaskOnlyInFailed checks the committed tree and the working tree after a
// successful fail: HEAD lists the task only under tasks/failed, and no task file
// shows an unstaged or staged change.
func assertTaskOnlyInFailed(t *testing.T, root string) {
	t.Helper()
	listing := runGit(t, root, "ls-tree", "-r", "HEAD", "--name-only")
	if !strings.Contains(listing, "tasks/failed/TF-001-example.md\n") {
		t.Fatalf("HEAD lacks tasks/failed entry:\n%s", listing)
	}
	for _, line := range strings.Split(listing, "\n") {
		if strings.HasPrefix(line, "tasks/active/TF-001") || strings.HasPrefix(line, "tasks/ready/TF-001") {
			t.Fatalf("HEAD still lists the task at %s", line)
		}
	}
	status := runGit(t, root, "status", "--short", "--", "tasks")
	if strings.TrimSpace(status) != "" {
		t.Fatalf("task files left dirty after fail: %q", status)
	}
}

// makeReadyTracked reproduces the claim state: the ready file is tracked in
// HEAD, the active file is untracked on disk, and the ready file is deleted in
// the working tree.
func makeReadyTracked(t *testing.T, root string) {
	t.Helper()
	active := filepath.Join(root, "tasks", "active", "TF-001-example.md")
	content := readFile(t, active)
	ready := filepath.Join(root, "tasks", "ready", "TF-001-example.md")
	writeFile(t, ready, string(content))
	runGit(t, root, "add", "tasks/ready/TF-001-example.md")
	runGit(t, root, "rm", "-q", "--cached", "tasks/active/TF-001-example.md")
	runGit(t, root, "commit", "-q", "-m", "ready state")
	if err := os.Remove(ready); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedStatesReadyTrackedActiveUntrackedStagesReadyDeletion(t *testing.T) {
	root := newProject(t, true)
	makeReadyTracked(t, root)
	writeEvidence(t, root, "FAILED")

	if _, err := Fail(root, "TF-001", Options{Outcome: "FAILED"}); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	assertTaskOnlyInFailed(t, root)
	names := runGit(t, root, "show", "--no-renames", "--name-status", "--format=", "HEAD")
	if !strings.Contains(names, "D\ttasks/ready/TF-001-example.md") || !strings.Contains(names, "A\ttasks/failed/TF-001-example.md") {
		t.Fatalf("commit paths = %q", names)
	}
}

func TestTrackedStatesActiveTrackedStagesActiveDeletion(t *testing.T) {
	root := newProject(t, true)
	writeEvidence(t, root, "BLOCKED")

	if _, err := Fail(root, "TF-001", Options{Outcome: "BLOCKED"}); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	assertTaskOnlyInFailed(t, root)
}

func TestTrackedStatesRefusesWhenNeitherTracked(t *testing.T) {
	root := newProject(t, true)
	runGit(t, root, "rm", "-q", "--cached", "tasks/active/TF-001-example.md")
	runGit(t, root, "commit", "-q", "-m", "untrack active")
	writeEvidence(t, root, "FAILED")
	before := commitCount(t, root)

	_, err := Fail(root, "TF-001", Options{Outcome: "FAILED"})
	if err == nil || !strings.Contains(err.Error(), "neither") {
		t.Fatalf("err = %v, want neither-tracked refusal", err)
	}
	if commitCount(t, root) != before {
		t.Fatal("refused command created a commit")
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "active", "TF-001-example.md")); err != nil {
		t.Fatalf("active file moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "failed", "TF-001-example.md")); !os.IsNotExist(err) {
		t.Fatalf("failed file created: %v", err)
	}
}

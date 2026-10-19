package requeue

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"taskfactory/internal/claim"
	"taskfactory/internal/fail"
)

// newClaimedFailed builds a repository where TF-001 was really claimed (a
// worktree under .taskfactory/worktrees and the branch task/TF-001 exist) and
// then failed, so tasks/failed holds it with a Claim block.
func newClaimedFailed(t *testing.T) (root, worktree string) {
	t.Helper()
	root = newProject(t, false)
	runGit(t, root, "mv", "tasks/failed/"+taskFile, "tasks/ready/"+taskFile)
	writeFile(t, filepath.Join(root, ".gitignore"), ".taskfactory/worktrees/\n.taskfactory/evidence/\n")
	runGit(t, root, "add", "--all")
	runGit(t, root, "commit", "-q", "-m", "ready")
	if err := claim.Claim(root, "TF-001", "test-worker"); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if _, err := fail.Fail(root, "TF-001", fail.Options{Outcome: "BLOCKED", Reason: "stuck", ReasonSet: true}); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	worktree = filepath.Join(root, ".taskfactory", "worktrees", "TF-001")
	if !exists(worktree) || !branchExists(t, root, "task/TF-001") {
		t.Fatal("claim did not create the worktree and branch")
	}
	return root, worktree
}

func branchExists(t *testing.T, root, branch string) bool {
	t.Helper()
	return strings.TrimSpace(runGit(t, root, "branch", "--list", branch)) != ""
}

// commitInWorktree commits one new file in the worktree and returns nothing.
func commitInWorktree(t *testing.T, worktree, name string) {
	t.Helper()
	writeFile(t, filepath.Join(worktree, name), "worker change\n")
	runGit(t, worktree, "add", "--all")
	runGit(t, worktree, "commit", "-q", "-m", "worker commit "+name)
}

// snapshot captures everything a refused requeue must leave alone: the task
// files, the tree of HEAD, the commit count, the status and every evidence file.
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(runGit(t, root, "ls-tree", "-r", "HEAD"))
	b.WriteString(runGit(t, root, "rev-list", "--count", "HEAD"))
	b.WriteString(runGit(t, root, "status", "--porcelain", "--untracked-files=all"))
	b.WriteString(runGit(t, root, "branch", "--list"))
	b.WriteString(runGit(t, root, "worktree", "list"))
	var names []string
	for _, state := range []string{"inbox", "ready", "active", "failed"} {
		entries, _ := os.ReadDir(filepath.Join(root, "tasks", state))
		for _, e := range entries {
			names = append(names, filepath.Join("tasks", state, e.Name()))
		}
	}
	evidence, _ := os.ReadDir(filepath.Join(root, ".taskfactory", "evidence"))
	for _, e := range evidence {
		names = append(names, filepath.Join(".taskfactory", "evidence", e.Name()))
	}
	sort.Strings(names)
	for _, name := range names {
		b.WriteString(name + "\n")
		b.Write(readFile(t, filepath.Join(root, name)))
	}
	return b.String()
}

func requireRefusal(t *testing.T, root string, opts Options, wants ...string) {
	t.Helper()
	before := snapshot(t, root)
	_, err := RequeueWith(root, "TF-001", "ready", opts)
	if err == nil {
		t.Fatal("requeue succeeded, want a refusal")
	}
	var ce CleanupError
	if errors.As(err, &ce) {
		t.Fatalf("got a cleanup failure after commit, want a refusal: %v", err)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
	if after := snapshot(t, root); after != before {
		t.Fatalf("refusal changed state:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func requireCleaned(t *testing.T, root, worktree string) {
	t.Helper()
	if exists(worktree) {
		t.Fatal("worktree directory still exists")
	}
	if branchExists(t, root, "task/TF-001") {
		t.Fatal("branch task/TF-001 still exists")
	}
	if list := strings.TrimSpace(runGit(t, root, "worktree", "list")); strings.Count(list, "\n") != 0 {
		t.Fatalf("worktree list not reduced to the main checkout:\n%s", list)
	}
	assertTaskOnlyIn(t, root, "ready")
}

func TestCleanupSafeRemovesAndReclaims(t *testing.T) {
	root, worktree := newClaimedFailed(t)
	writeEvidence(t, root, 1)
	// Before this change the re-claim below failed: the branch already exists.
	res, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true})
	if err != nil {
		t.Fatalf("RequeueWith: %v", err)
	}
	if res.RemovedWorktree != worktree || res.RemovedBranch != "task/TF-001" {
		t.Fatalf("result = %+v", res)
	}
	requireCleaned(t, root, worktree)
	if err := claim.Claim(root, "TF-001", "second-worker"); err != nil {
		t.Fatalf("re-claim after requeue failed: %v", err)
	}
	if !exists(worktree) || !branchExists(t, root, "task/TF-001") {
		t.Fatal("re-claim did not recreate the worktree and branch")
	}
}

func TestCleanupSafeRemovesAndReclaimsConventionalWithoutClaim(t *testing.T) {
	root, worktree := newClaimedFailed(t)
	data := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	stripped, _ := stripClaim(data)
	writeFile(t, filepath.Join(root, "tasks", "failed", taskFile), string(stripped))
	runGit(t, root, "add", "--all")
	runGit(t, root, "commit", "-q", "-m", "drop claim block by hand")
	res, err := RequeueWith(root, "TF-001", "ready", Options{})
	if err != nil {
		t.Fatalf("RequeueWith: %v", err)
	}
	if res.RemovedWorktree != worktree || res.RemovedBranch != "task/TF-001" {
		t.Fatalf("result = %+v", res)
	}
	requireCleaned(t, root, worktree)
	if err := claim.Claim(root, "TF-001", "second-worker"); err != nil {
		t.Fatalf("re-claim after requeue failed: %v", err)
	}
}

func TestCleanupDirtyRefuses(t *testing.T) {
	for name, dirty := range map[string]func(t *testing.T, worktree string){
		"untracked": func(t *testing.T, w string) { writeFile(t, filepath.Join(w, "scratch.txt"), "x\n") },
		"modified":  func(t *testing.T, w string) { writeFile(t, filepath.Join(w, "tasks", "failed", ".gitkeep"), "x\n") },
	} {
		t.Run(name, func(t *testing.T) {
			root, worktree := newClaimedFailed(t)
			writeEvidence(t, root, 2)
			dirty(t, worktree)
			requireRefusal(t, root, Options{DiscardClaim: true},
				"TF-001", "uncommitted or untracked", worktree,
				"git worktree remove "+worktree, "git branch -d task/TF-001", "git branch -D task/TF-001", "Nothing was changed")
			if !exists(worktree) || !branchExists(t, root, "task/TF-001") {
				t.Fatal("refusal removed the worktree or branch")
			}
		})
	}
}

func TestCleanupUnmergedRefuses(t *testing.T) {
	for _, count := range []string{"1", "2"} {
		t.Run(count, func(t *testing.T) {
			root, worktree := newClaimedFailed(t)
			writeEvidence(t, root, 1)
			commitInWorktree(t, worktree, "a.txt")
			if count == "2" {
				commitInWorktree(t, worktree, "b.txt")
			}
			requireRefusal(t, root, Options{DiscardClaim: true},
				count+" unmerged commit(s)", "git log --oneline main..task/TF-001",
				"git worktree remove "+worktree, "git branch -d task/TF-001", "git branch -D task/TF-001", "Nothing was changed")
			if !exists(worktree) || !branchExists(t, root, "task/TF-001") {
				t.Fatal("refusal removed the worktree or branch")
			}
		})
	}
}

func TestCleanupMergedBranch(t *testing.T) {
	root, worktree := newClaimedFailed(t)
	commitInWorktree(t, worktree, "merged.txt")
	runGit(t, root, "merge", "-q", "--no-edit", "task/TF-001")
	if _, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true}); err != nil {
		t.Fatalf("RequeueWith: %v", err)
	}
	requireCleaned(t, root, worktree)
	if !exists(filepath.Join(root, "merged.txt")) {
		t.Fatal("merged work was lost from main")
	}
}

func TestCleanupBaseCommitBranch(t *testing.T) {
	root, worktree := newClaimedFailed(t)
	// Main moves on; the task branch stays at its Claim base commit.
	writeFile(t, filepath.Join(root, "other.txt"), "other\n")
	runGit(t, root, "add", "other.txt")
	runGit(t, root, "commit", "-q", "-m", "main moves on")
	base := strings.TrimSpace(runGit(t, root, "rev-parse", "task/TF-001"))
	if !strings.Contains(string(readFile(t, filepath.Join(root, "tasks", "failed", taskFile))), "Base commit: "+base) {
		t.Fatal("branch is not at the Claim base commit")
	}
	if _, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true}); err != nil {
		t.Fatalf("RequeueWith: %v", err)
	}
	requireCleaned(t, root, worktree)
}

func TestCleanupOutsideRoot(t *testing.T) {
	outsideDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(root, original string) string{
		"elsewhere":     func(root, original string) string { return filepath.Join(outsideDir, "wt") },
		"project root":  func(root, original string) string { return root },
		"worktree root": func(root, original string) string { return filepath.Join(root, ".taskfactory", "worktrees") },
		"dotdot":        func(root, original string) string { return filepath.Join(original, "..", "..", "..") },
	} {
		t.Run(name, func(t *testing.T) {
			root, worktree := newClaimedFailed(t)
			outside := mutate(root, worktree)
			if name == "elsewhere" {
				runGit(t, root, "worktree", "add", "-q", outside, "-b", "other-branch")
			}
			path := filepath.Join(root, "tasks", "failed", taskFile)
			data := strings.Replace(string(readFile(t, path)), "Worktree: "+worktree, "Worktree: "+outside, 1)
			writeFile(t, path, data)
			runGit(t, root, "add", "--all")
			runGit(t, root, "commit", "-q", "-m", "point claim elsewhere")
			requireRefusal(t, root, Options{DiscardClaim: true}, "outside", "never touches", "git worktree remove", "git branch -d task/TF-001")
			if !exists(outside) || !exists(worktree) || !branchExists(t, root, "task/TF-001") {
				t.Fatal("a path or branch was removed")
			}
		})
	}
}

func TestCleanupMissing(t *testing.T) {
	t.Run("both missing", func(t *testing.T) {
		root, worktree := newClaimedFailed(t)
		runGit(t, root, "worktree", "remove", worktree)
		runGit(t, root, "branch", "-d", "task/TF-001")
		res, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true})
		if err != nil {
			t.Fatalf("RequeueWith: %v", err)
		}
		if res.RemovedWorktree != "" || res.RemovedBranch != "" {
			t.Fatalf("result = %+v, nothing was there to remove", res)
		}
		assertTaskOnlyIn(t, root, "ready")
	})
	t.Run("worktree missing, branch present", func(t *testing.T) {
		root, worktree := newClaimedFailed(t)
		runGit(t, root, "worktree", "remove", worktree)
		res, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true})
		if err != nil {
			t.Fatalf("RequeueWith: %v", err)
		}
		if res.RemovedWorktree != "" || res.RemovedBranch != "task/TF-001" {
			t.Fatalf("result = %+v", res)
		}
		requireCleaned(t, root, worktree)
	})
	t.Run("directory deleted by hand", func(t *testing.T) {
		root, worktree := newClaimedFailed(t)
		if err := os.RemoveAll(worktree); err != nil {
			t.Fatal(err)
		}
		if _, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true}); err != nil {
			t.Fatalf("RequeueWith: %v", err)
		}
		requireCleaned(t, root, worktree)
	})
	t.Run("branch missing, worktree on a detached HEAD", func(t *testing.T) {
		root, worktree := newClaimedFailed(t)
		runGit(t, worktree, "checkout", "-q", "--detach")
		runGit(t, root, "branch", "-d", "task/TF-001")
		res, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true})
		if err != nil {
			t.Fatalf("RequeueWith: %v", err)
		}
		if res.RemovedWorktree != worktree || res.RemovedBranch != "" {
			t.Fatalf("result = %+v", res)
		}
		requireCleaned(t, root, worktree)
	})
}

func TestCleanupFailureAfterCommit(t *testing.T) {
	root, worktree := newClaimedFailed(t)
	writeEvidence(t, root, 1)
	// A locked worktree passes the safety checks but git worktree remove,
	// run without force, refuses it. That is a real failure after the commit.
	runGit(t, root, "worktree", "lock", worktree)
	res, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true})
	var ce CleanupError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want a CleanupError", err)
	}
	for _, want := range []string{"WAS requeued and committed", "Left behind", "worktree " + worktree, "branch task/TF-001", "git worktree remove " + worktree, "git branch -d task/TF-001"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
	if res.Path != "tasks/ready/"+taskFile || res.RemovedWorktree != "" || res.RemovedBranch != "" {
		t.Fatalf("result = %+v", res)
	}
	if subject := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); subject != "docs: requeue TF-001 to ready" {
		t.Fatalf("requeue commit missing, HEAD subject = %q", subject)
	}
	assertTaskOnlyIn(t, root, "ready")
	if !exists(worktree) || !branchExists(t, root, "task/TF-001") {
		t.Fatal("the failed cleanup should have left both behind")
	}
}

func TestCleanupKeptWhenRefusedElsewhere(t *testing.T) {
	root, worktree := newClaimedFailed(t)
	// Without --discard-claim the claimed task is refused: nothing is removed.
	requireRefusal(t, root, Options{}, "flagged for a human decision")
	// An invalid ready contract is refused after the safety checks passed.
	path := filepath.Join(root, "tasks", "failed", taskFile)
	broken := strings.Replace(string(readFile(t, path)), "Check: false", "No check here", 1)
	writeFile(t, path, broken)
	runGit(t, root, "add", "--all")
	runGit(t, root, "commit", "-q", "-m", "break contract")
	requireRefusal(t, root, Options{DiscardClaim: true}, "refusing to requeue")
	if !exists(worktree) || !branchExists(t, root, "task/TF-001") {
		t.Fatal("a refused requeue removed the worktree or branch")
	}
	if !bytes.Contains(readFile(t, path), []byte("## Claim")) {
		t.Fatal("claim block lost")
	}
}

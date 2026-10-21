package requeue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/claim"
	"taskfactory/internal/fail"
)

// newClaimedFailedOn is newClaimedFailed for a project whose base branch is
// name, set in the committed config before the claim.
func newClaimedFailedOn(t *testing.T, name string) (root, worktree string) {
	t.Helper()
	root = newProject(t, false)
	runGit(t, root, "branch", "-m", "main", name)
	p := filepath.Join(root, ".taskfactory", "config.toml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(b), "[git]\n", "[git]\nbase_branch = \""+name+"\"\n", 1)
	if updated == string(b) {
		t.Fatal("[git] table not found in fixture config")
	}
	writeFile(t, p, updated)
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
	return root, filepath.Join(root, ".taskfactory", "worktrees", "TF-001")
}

func TestCleanupBaseBranchMasterRemoves(t *testing.T) {
	t.Parallel()
	root, worktree := newClaimedFailedOn(t, "master")
	writeEvidence(t, root, 1)
	if _, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true}); err != nil {
		t.Fatalf("RequeueWith: %v", err)
	}
	requireCleaned(t, root, worktree)
}

func TestCleanupBaseBranchMasterRefusesUnmerged(t *testing.T) {
	t.Parallel()
	root, worktree := newClaimedFailedOn(t, "master")
	commitInWorktree(t, worktree, "extra.txt")
	requireRefusal(t, root, Options{DiscardClaim: true}, "not on master", "master..task/TF-001")
}

func TestCleanupBaseBranchMissingNamesBranch(t *testing.T) {
	t.Parallel()
	root, _ := newClaimedFailedOn(t, "master")
	runGit(t, root, "checkout", "-q", "--detach")
	runGit(t, root, "branch", "-m", "master", "elsewhere")
	requireRefusal(t, root, Options{DiscardClaim: true}, "local master", "branch master does not exist")
}

package claim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useBaseBranch renames the fixture's main branch to name and, when configure
// is true, sets git.base_branch in the committed config.
func useBaseBranch(t *testing.T, name string, configure bool) string {
	t.Helper()
	root := newGitProject(t, 4, "TF-001")
	gitRun(t, root, "branch", "-m", "main", name)
	if configure {
		p := filepath.Join(root, ".taskfactory", "config.toml")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		updated := strings.Replace(string(b), "[git]\n", "[git]\nbase_branch = \""+name+"\"\n", 1)
		if err = os.WriteFile(p, []byte(updated), 0o600); err != nil {
			t.Fatal(err)
		}
		gitRun(t, root, "add", "--all")
		gitRun(t, root, "commit", "--quiet", "-m", "base branch")
	}
	return root
}

func TestClaimBaseBranchMaster(t *testing.T) {
	t.Parallel()
	root := useBaseBranch(t, "master", true)
	want := gitOutput(t, root, "rev-parse", "refs/heads/master")
	if err := Claim(root, "TF-001", "worker"); err != nil {
		t.Fatalf("Claim on master: %v", err)
	}
	if got := gitOutput(t, root, "rev-parse", "refs/heads/task/TF-001"); got != want {
		t.Fatalf("task branch at %s, want master %s", got, want)
	}
	data, err := os.ReadFile(filepath.Join(root, "tasks", "active", "TF-001-target.md"))
	if err != nil || !strings.Contains(string(data), "Base commit: "+want) {
		t.Fatalf("claim block lacks master commit: %v\n%s", err, data)
	}
}

func TestClaimBaseBranchMissingNamesBranch(t *testing.T) {
	t.Parallel()
	root := useBaseBranch(t, "master", false) // renamed, but the config says nothing
	err := Claim(root, "TF-001", "worker")
	if err == nil || !strings.Contains(err.Error(), "refs/heads/main") {
		t.Fatalf("Claim error = %v, want a refusal naming refs/heads/main", err)
	}
}

func TestClaimBaseBranchConfiguredButAbsentNamesBranch(t *testing.T) {
	t.Parallel()
	root := useBaseBranch(t, "master", true)
	gitRun(t, root, "checkout", "-q", "--detach")
	gitRun(t, root, "branch", "-m", "master", "elsewhere")
	err := Claim(root, "TF-001", "worker")
	if err == nil || !strings.Contains(err.Error(), "refs/heads/master") {
		t.Fatalf("Claim error = %v, want a refusal naming refs/heads/master", err)
	}
}

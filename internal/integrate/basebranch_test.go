package integrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useBaseBranch renames the fixture's main branch to name, sets
// git.base_branch in the committed config and returns the fixture.
func useBaseBranch(t *testing.T, name string) fixture {
	t.Helper()
	f := newFixture(t)
	run(t, f.root, "git", "branch", "-m", "main", name)
	p := filepath.Join(f.root, ".taskfactory/config.toml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(b), "[git]\n", "[git]\nbase_branch=\""+name+"\"\n", 1)
	if updated == string(b) {
		t.Fatal("[git] table not found in fixture config")
	}
	if err = os.WriteFile(p, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, f.root, "git", "add", "--", ".taskfactory/config.toml")
	run(t, f.root, "git", "commit", "-m", "set base branch")
	return f
}

func TestBaseBranchIntegrateOnMaster(t *testing.T) {
	t.Parallel()
	f := useBaseBranch(t, "master")
	if err := Run(f.root, "TF-901"); err != nil {
		evidence, _ := os.ReadFile(filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl"))
		t.Fatalf("Run: %v; evidence=%s", err, evidence)
	}
	// The config commit made the candidate rebase, so compare the subject.
	if subject, _ := git(f.root, "log", "-1", "--format=%s", "master^"); subject != "feature" {
		t.Fatalf("master^ subject=%q, want the rebased candidate commit", subject)
	}
	if branch, _ := git(f.root, "branch", "--show-current"); branch != "master" {
		t.Fatalf("checked out %q, want master", branch)
	}
	if _, err := os.Stat(filepath.Join(f.root, "tasks/archive/TF-901-example.md")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(f.root, ".taskfactory/integration-evidence/TF-901.jsonl"))
	if err != nil || !strings.Contains(string(data), `"outcome":"PASS"`) || strings.Contains(string(data), `"main_after":""`) {
		t.Fatalf("evidence=%s err=%v", data, err)
	}
}

func TestBaseBranchDryRunNamesBranch(t *testing.T) {
	t.Parallel()
	f := useBaseBranch(t, "trunk")
	var steps []string
	err := RunWith(f.root, "TF-901", Options{DryRun: true, Would: func(format string, args ...any) {
		steps = append(steps, strings.TrimSpace(fmt.Sprintf(format, args...)))
	}})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(steps, "\n")
	if !strings.Contains(joined, "onto trunk") || !strings.Contains(joined, "fast-forward trunk") {
		t.Fatalf("dry run steps do not name trunk:\n%s", joined)
	}
}

func TestBaseBranchNotCheckedOutNamesBranch(t *testing.T) {
	t.Parallel()
	f := useBaseBranch(t, "master")
	run(t, f.root, "git", "checkout", "-q", "-b", "other")
	err := Run(f.root, "TF-901")
	if err == nil || !strings.Contains(err.Error(), "local master checked out") {
		t.Fatalf("Run error=%v", err)
	}
}

func TestBaseBranchMissingBranchNamesBranch(t *testing.T) {
	t.Parallel()
	f := useBaseBranch(t, "master")
	run(t, f.root, "git", "checkout", "-q", "--detach")
	run(t, f.root, "git", "branch", "-m", "master", "elsewhere")
	err := Run(f.root, "TF-901")
	if err == nil || !strings.Contains(err.Error(), "refs/heads/master is unavailable") {
		t.Fatalf("Run error=%v", err)
	}
}

func TestBaseBranchCheckMainOnMaster(t *testing.T) {
	t.Parallel()
	f := useBaseBranch(t, "master")
	setMainChecks(t, f.root, "false")
	if err := Run(f.root, "TF-901"); err == nil {
		t.Fatal("Run unexpectedly succeeded")
	}
	mustStop(t, f.root)
	setMainChecks(t, f.root, "true")
	if err := CheckMain(f.root); err != nil {
		t.Fatalf("CheckMain: %v", err)
	}
	if _, ok := readOptional(t, stopPath(f.root)); ok {
		t.Fatal("stop record remains after passing checks on master")
	}
	// With another branch checked out, the error names the configured branch.
	run(t, f.root, "git", "checkout", "-q", "-b", "other")
	if err := CheckMain(f.root); err == nil || !strings.Contains(err.Error(), "local master checked out") {
		t.Fatalf("CheckMain error=%v", err)
	}
}

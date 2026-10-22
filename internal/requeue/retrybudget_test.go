package requeue

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRequeueRefusesPastRetryBudget(t *testing.T) {
	t.Parallel()
	root := newProject(t, false)
	writeFile(t, filepath.Join(root, ".taskfactory", "config.toml"), "retry_budget = 2\n"+testConfig)
	evidence := writeEvidence(t, root, 2)
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	before := commitCount(t, root)

	_, err := Requeue(root, "TF-001", "ready")
	if err == nil {
		t.Fatal("Requeue succeeded past the retry budget")
	}
	for _, want := range []string{"TF-001", "used 2 attempt(s)", "retry budget is 2"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
	assertUnchanged(t, root, original, before)
	if !bytes.Equal(readFile(t, filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")), evidence) {
		t.Fatal("refused requeue changed the evidence file")
	}
	if exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-2.jsonl")) {
		t.Fatal("refused requeue renamed the evidence file aside")
	}
}

func TestRequeueAllowedBelowRetryBudget(t *testing.T) {
	t.Parallel()
	root := newProject(t, false)
	writeFile(t, filepath.Join(root, ".taskfactory", "config.toml"), "retry_budget = 3\n"+testConfig)
	writeEvidence(t, root, 2)

	if _, err := Requeue(root, "TF-001", "ready"); err != nil {
		t.Fatalf("Requeue below the budget: %v", err)
	}
	assertTaskOnlyIn(t, root, "ready")
}

// failAgain moves the requeued task back to tasks/failed and commits the move,
// as a worker failing the task would leave it.
func failAgain(t *testing.T, root string) {
	t.Helper()
	if err := os.Rename(filepath.Join(root, "tasks", "ready", taskFile), filepath.Join(root, "tasks", "failed", taskFile)); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "--all", "tasks")
	runGit(t, root, "commit", "-q", "-m", "test: fail TF-001 again")
}

func TestRequeueBudgetCountsAcrossCycles(t *testing.T) {
	t.Parallel()
	root := newProject(t, false)
	writeFile(t, filepath.Join(root, ".taskfactory", "config.toml"), "retry_budget = 6\n"+testConfig)

	// Each cycle uses 2 attempts, which is below the budget of 6 on its own.
	// The totals counted through the cycles are 2, then 4. The third requeue
	// would reach 6 and must be refused.
	for cycle := 1; cycle <= 2; cycle++ {
		writeEvidence(t, root, 2)
		if _, err := Requeue(root, "TF-001", "ready"); err != nil {
			t.Fatalf("cycle %d: Requeue below the budget: %v", cycle, err)
		}
		assertTaskOnlyIn(t, root, "ready")
		if !exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-"+strconv.Itoa(2*cycle)+".jsonl")) {
			t.Fatalf("cycle %d: aside file for %d total attempts is missing", cycle, 2*cycle)
		}
		failAgain(t, root)
	}

	writeEvidence(t, root, 2)
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	before := commitCount(t, root)
	_, err := Requeue(root, "TF-001", "ready")
	if err == nil {
		t.Fatal("Requeue succeeded once the total attempts reached the retry budget")
	}
	for _, want := range []string{"TF-001", "used 6 attempt(s)", "retry budget is 6"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
	assertUnchanged(t, root, original, before)
	if exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-6.jsonl")) {
		t.Fatal("refused requeue renamed the evidence file aside")
	}
}

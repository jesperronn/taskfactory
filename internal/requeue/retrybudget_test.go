package requeue

import (
	"bytes"
	"path/filepath"
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

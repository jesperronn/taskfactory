package requeue

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscardClaimToReady(t *testing.T) {
	root := newProject(t, true)
	res, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true})
	if err != nil {
		t.Fatalf("RequeueWith: %v", err)
	}
	if res.Path != "tasks/ready/"+taskFile || res.Discarded == "" {
		t.Fatalf("result = %+v", res)
	}
	got := readFile(t, filepath.Join(root, "tasks", "ready", taskFile))
	if bytes.Contains(got, []byte("## Claim")) || bytes.Contains(got, []byte("Owner:")) {
		t.Fatalf("claim block still present:\n%s", got)
	}
	if exists(filepath.Join(root, "tasks", "failed", taskFile)) {
		t.Fatal("failed file still exists")
	}
	if subject := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); subject != "docs: requeue TF-001 to ready" {
		t.Fatalf("subject = %q", subject)
	}
	names := runGit(t, root, "show", "--no-renames", "--name-status", "--format=", "HEAD")
	if !strings.Contains(names, "D\ttasks/failed/"+taskFile) || !strings.Contains(names, "A\ttasks/ready/"+taskFile) || strings.Count(names, "\n") != 2 {
		t.Fatalf("commit paths = %q", names)
	}
	assertTaskOnlyIn(t, root, "ready")
}

func TestDiscardClaimToInbox(t *testing.T) {
	root := newProject(t, true)
	// An inbox task needs no contract check, so a broken contract is accepted.
	writeFile(t, filepath.Join(root, "tasks", "failed", taskFile), "# TF-001: Rough idea\n\nNotes.\n"+claimBlock("TF-001", root))
	runGit(t, root, "add", "--all")
	runGit(t, root, "commit", "-q", "-m", "rough failed task")
	res, err := RequeueWith(root, "TF-001", "inbox", Options{DiscardClaim: true})
	if err != nil {
		t.Fatalf("RequeueWith: %v", err)
	}
	if res.Path != "tasks/inbox/"+taskFile {
		t.Fatalf("result = %+v", res)
	}
	if got := string(readFile(t, filepath.Join(root, "tasks", "inbox", taskFile))); got != "# TF-001: Rough idea\n\nNotes.\n" {
		t.Fatalf("inbox file = %q", got)
	}
	assertTaskOnlyIn(t, root, "inbox")
}

func TestDiscardClaimBodyRecordsClaim(t *testing.T) {
	root := newProject(t, true)
	if _, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true}); err != nil {
		t.Fatal(err)
	}
	body := runGit(t, root, "log", "-1", "--format=%B")
	for _, want := range []string{
		"Discarded claim",
		"Owner: test-worker",
		"Branch: task/TF-001-impl",
		"Worktree: " + filepath.Join(root, ".taskfactory", "worktrees", "TF-001"),
		"Base commit: " + testBaseCommit,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("commit message lacks %q:\n%s", want, body)
		}
	}
	// The recorded block carries the values exactly as the block had them.
	if !strings.Contains(body, "## Claim\n\nOwner: test-worker\nBranch:") {
		t.Errorf("claim block not recorded verbatim:\n%s", body)
	}
}

func TestDiscardClaimNeededWithoutFlagIsRefusedAndNamesFlag(t *testing.T) {
	root := newProject(t, true)
	writeEvidence(t, root, 1)
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	before := commitCount(t, root)
	for _, target := range []string{"ready", "inbox"} {
		_, err := RequeueWith(root, "TF-001", target, Options{})
		if err == nil {
			t.Fatalf("--to %s: claimed task was requeued without the flag", target)
		}
		for _, want := range []string{"flagged for a human decision", "TF-001", "--discard-claim"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q lacks %q", err, want)
			}
		}
		assertUnchanged(t, root, original, before)
	}
	if !exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")) {
		t.Fatal("evidence touched by a refusal")
	}
}

func TestDiscardClaimKeepsRestOfFileAndRenamesEvidence(t *testing.T) {
	root := newProject(t, true)
	evidence := writeEvidence(t, root, 3)
	if _, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true}); err != nil {
		t.Fatal(err)
	}
	if got := string(readFile(t, filepath.Join(root, "tasks", "ready", taskFile))); got != contract("TF-001") {
		t.Fatalf("rest of the file changed:\n%q\nwant\n%q", got, contract("TF-001"))
	}
	if exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")) {
		t.Fatal("old evidence name still present: counter not reset")
	}
	if !bytes.Equal(readFile(t, filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-3.jsonl")), evidence) {
		t.Fatal("aside evidence differs from the original bytes")
	}
	if body := runGit(t, root, "log", "-1", "--format=%B"); !strings.Contains(body, "TF-001.attempts-3.jsonl") {
		t.Fatalf("body does not name the aside file:\n%s", body)
	}
	if staged := runGit(t, root, "show", "--name-only", "--format=", "HEAD"); strings.Contains(staged, ".taskfactory") {
		t.Fatalf("evidence was committed: %s", staged)
	}
}

func TestDiscardClaimKeepsTextAfterClaimBlock(t *testing.T) {
	data := []byte(contract("TF-001") + "\n## Claim\n\nOwner: a\nBranch: b\n\n## Outcome\n\nKept.\n")
	got, removed := stripClaim(data)
	if string(got) != contract("TF-001")+"\n## Outcome\n\nKept.\n" {
		t.Fatalf("stripClaim = %q", got)
	}
	if removed != "## Claim\n\nOwner: a\nBranch: b" {
		t.Fatalf("removed = %q", removed)
	}
}

func TestDiscardClaimUnclaimedTaskIsANoOpNote(t *testing.T) {
	root := newProject(t, false)
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	res, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true})
	if err != nil {
		t.Fatalf("RequeueWith: %v", err)
	}
	if res.Discarded != "" {
		t.Fatalf("Discarded = %q for an unclaimed task", res.Discarded)
	}
	if !bytes.Equal(readFile(t, filepath.Join(root, "tasks", "ready", taskFile)), original) {
		t.Fatal("task bytes changed")
	}
	if body := runGit(t, root, "log", "-1", "--format=%B"); strings.Contains(body, "Discarded claim") {
		t.Fatalf("body claims a discard:\n%s", body)
	}
	assertTaskOnlyIn(t, root, "ready")
}

func TestDiscardClaimRestoresWhenCommitFails(t *testing.T) {
	root := newProject(t, true)
	evidence := writeEvidence(t, root, 2)
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	writeFile(t, hook, "#!/bin/sh\necho rejected by test hook >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	before := commitCount(t, root)
	_, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true})
	if err == nil || !strings.Contains(err.Error(), "commit requeue") {
		t.Fatalf("err = %v, want commit failure", err)
	}
	assertUnchanged(t, root, original, before)
	if !bytes.Equal(readFile(t, filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")), evidence) || exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-2.jsonl")) {
		t.Fatal("evidence not restored under its original name")
	}
	if status := runGit(t, root, "status", "--short"); strings.TrimSpace(status) != "" && !strings.Contains(status, ".taskfactory") {
		t.Fatalf("status not clean: %q", status)
	}
}

func TestDiscardClaimRestoresWhenValidationFails(t *testing.T) {
	root := newProject(t, true)
	writeFile(t, filepath.Join(root, "tasks", "failed", taskFile), strings.Replace(contract("TF-001"), "Check: false", "No check here", 1)+claimBlock("TF-001", root))
	runGit(t, root, "add", "--all")
	runGit(t, root, "commit", "-q", "-m", "invalid contract")
	writeEvidence(t, root, 1)
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	before := commitCount(t, root)
	_, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true})
	if err == nil {
		t.Fatal("invalid ready contract was accepted")
	}
	assertUnchanged(t, root, original, before)
	if !exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.jsonl")) || exists(filepath.Join(root, ".taskfactory", "evidence", "TF-001.attempts-1.jsonl")) {
		t.Fatal("evidence renamed despite the refusal")
	}
}

func TestDiscardClaimRestoresUntrackedFailedFile(t *testing.T) {
	root := newProject(t, true)
	runGit(t, root, "rm", "-q", "--cached", "tasks/failed/"+taskFile)
	runGit(t, root, "commit", "-q", "-m", "untrack")
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	writeFile(t, hook, "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	original := readFile(t, filepath.Join(root, "tasks", "failed", taskFile))
	before := commitCount(t, root)
	if _, err := RequeueWith(root, "TF-001", "ready", Options{DiscardClaim: true}); err == nil {
		t.Fatal("commit hook failure was not reported")
	}
	assertUnchanged(t, root, original, before)
}

func TestDiscardClaimParseOptions(t *testing.T) {
	id, to, opts, err := ParseOptions([]string{"--discard-claim", "TF-001", "--to=inbox"})
	if err != nil || id != "TF-001" || to != "inbox" || !opts.DiscardClaim {
		t.Fatalf("ParseOptions = %q %q %+v %v", id, to, opts, err)
	}
	for _, args := range [][]string{
		{"TF-001", "--discard-claim=yes"},
		{"TF-001", "--discard-claim", "--discard-claim"},
		{"TF-001", "--discard"},
	} {
		if _, _, _, err := ParseOptions(args); err == nil {
			t.Errorf("ParseOptions(%v) succeeded", args)
		} else if _, ok := err.(UsageError); !ok {
			t.Errorf("ParseOptions(%v) error %T is not UsageError", args, err)
		}
	}
}

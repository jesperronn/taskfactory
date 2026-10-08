package taskvalidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validTask = `# TF-101: Example task

## Goal

Do the work.

## Dependencies

None

## Scope

Implement it.

## Constraints

Keep it small.

## Success criteria

### C1: It works

Check: go test ./...

## Verification

Run the check.
`

func TestValidateAcceptsCompleteTaskAndReportsStableFieldDiagnostics(t *testing.T) {
	root := project(t)
	writeTask(t, root, "ready", "TF-101-example.md", validTask)
	if got := Validate(root, ""); len(got) != 0 {
		t.Fatalf("valid task diagnostics = %#v", got)
	}
	writeTask(t, root, "ready", "TF-102-broken.md", strings.Replace(validTask, "Check: go test ./...", "Check:", 1))
	got := Validate(root, filepath.Join("tasks", "ready", "TF-102-broken.md"))
	if len(got) == 0 || !strings.Contains(got[0].Error(), "Check") {
		t.Fatalf("single-file diagnostics = %#v, want Check field", got)
	}
}

func TestValidateChecksTreeUniquenessAndDependencyResolutionForSelectedFile(t *testing.T) {
	root := project(t)
	selected := strings.Replace(validTask, "None", "- TF-102", 1)
	writeTask(t, root, "ready", "TF-101-selected.md", selected)
	writeTask(t, root, "inbox", "TF-101-duplicate.md", "# TF-101: Proposal\n\nunfinished\n")
	got := Validate(root, filepath.Join("tasks", "ready", "TF-101-selected.md"))
	if len(got) != 2 {
		t.Fatalf("diagnostics = %#v, want duplicate ID and unresolved dependency", got)
	}
	if !strings.Contains(got[0].Error(), "duplicate") || !strings.Contains(got[1].Error(), "dependency") {
		t.Fatalf("diagnostics not stable/field-specific: %#v", got)
	}
}

func TestValidateAcceptsLegacyArchiveAndInboxProposal(t *testing.T) {
	root := project(t)
	legacy := strings.Replace(validTask, "### C1: It works\n\nCheck: go test ./...", "- Historical criterion without check", 1)
	writeTask(t, root, "archive", "TF-101-legacy.md", legacy)
	writeTask(t, root, "inbox", "TF-102-proposal.md", "# TF-102: Proposal\n\nAny draft text.\n")
	if got := Validate(root, ""); len(got) != 0 {
		t.Fatalf("legacy and inbox diagnostics = %#v", got)
	}
}

func TestValidateIsReadOnly(t *testing.T) {
	root := project(t)
	path := writeTask(t, root, "ready", "TF-101-example.md", validTask)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = Validate(root, "")
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("validation modified task file")
	}
}

func TestValidateRejectsDependencyCyclesAndUnknownStates(t *testing.T) {
	root := project(t)
	a := strings.Replace(validTask, "None", "- TF-102", 1)
	b := strings.Replace(strings.Replace(validTask, "TF-101", "TF-102", 1), "None", "- TF-101", 1)
	writeTask(t, root, "ready", "TF-101-a.md", a)
	writeTask(t, root, "ready", "TF-102-b.md", b)
	if err := os.MkdirAll(filepath.Join(root, "tasks", "planned"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTask(t, root, "planned", "TF-103-other.md", "# TF-103: Other\n")
	got := Validate(root, "")
	joined := ""
	for _, d := range got {
		joined += d.Error() + "\n"
	}
	if !strings.Contains(joined, "dependency cycle") || !strings.Contains(joined, "state") {
		t.Fatalf("diagnostics = %s, want cycle and state errors", joined)
	}
}

func TestValidateEnforcesActiveClaimAndRejectsUnsupportedMarkdown(t *testing.T) {
	root := project(t)
	active := strings.Replace(validTask, "## Verification", "## Verification", 1) + "\n## Claim\n\nOwner: worker\nBranch: feature/task\nWorktree: /tmp/TF-101\nBase commit: abc\nStarted at: yesterday\n"
	active = strings.Replace(active, "Keep it small.", "<b>Keep it small.</b>", 1)
	writeTask(t, root, "active", "TF-101-active.md", active)
	got := Validate(root, "")
	joined := ""
	for _, d := range got {
		joined += d.Error() + "\n"
	}
	if !strings.Contains(joined, "unsupported Markdown") || !strings.Contains(joined, "Base commit") || !strings.Contains(joined, "Started at") {
		t.Fatalf("diagnostics = %s, want markdown and claim errors", joined)
	}
}

func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, state := range []string{"inbox", "ready", "active", "failed", "archive"} {
		if err := os.MkdirAll(filepath.Join(root, "tasks", state), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeTask(t *testing.T, root, state, name, content string) string {
	t.Helper()
	path := filepath.Join(root, "tasks", state, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

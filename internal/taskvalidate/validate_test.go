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
	writeTask(t, root, "planned", "TF-104-empty.md", "")
	writeTask(t, root, "planned", ".gitkeep", "")
	got := Validate(root, "")
	joined := ""
	for _, d := range got {
		joined += d.Error() + "\n"
	}
	if !strings.Contains(joined, "dependency cycle") || !strings.Contains(joined, "TF-103-other.md: state") || !strings.Contains(joined, "TF-104-empty.md: state") {
		t.Fatalf("diagnostics = %s, want cycle and both state errors", joined)
	}
	if strings.Contains(joined, ".gitkeep: state") {
		t.Fatalf("empty placeholder should be ignored: %s", joined)
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

func TestValidateRejectsEmptyTaskFileAndWhitespaceOnlyText(t *testing.T) {
	root := project(t)
	writeTask(t, root, "ready", "TF-101-empty.md", "")
	title := strings.Replace(validTask, "# TF-101: Example task", "# TF-101:    ", 1)
	writeTask(t, root, "ready", "TF-102-title.md", strings.Replace(title, "TF-101", "TF-102", 1))
	criterion := strings.Replace(validTask, "### C1: It works", "### C1:    ", 1)
	writeTask(t, root, "ready", "TF-103-criterion.md", strings.Replace(criterion, "TF-101", "TF-103", 1))
	joined := diagnosticsText(Validate(root, ""))
	for _, want := range []string{"TF-101-empty.md: heading", "TF-102-title.md: title", "TF-103-criterion.md: Success criteria"} {
		if !strings.Contains(joined, want) {
			t.Errorf("diagnostics missing %q:\n%s", want, joined)
		}
	}
}

func TestValidateRejectsDuplicateClaimKeyAfterEmptyValue(t *testing.T) {
	root := project(t)
	active := strings.Replace(validTask, "TF-101", "TF-104", 1) + "\n## Claim\n\nOwner: \nOwner: worker\nBranch: feature/task\nWorktree: /tmp/TF-104\nBase commit: abc\nStarted at: yesterday\n"
	writeTask(t, root, "active", "TF-104-claim.md", active)
	joined := diagnosticsText(Validate(root, ""))
	if !strings.Contains(joined, "Owner: must appear once") {
		t.Fatalf("diagnostics missing duplicate Owner key:\n%s", joined)
	}
}

func TestValidateRejectsUnknownMarkdownHeadingsInProseFields(t *testing.T) {
	root := project(t)
	content := strings.Replace(validTask, "Implement it.", "### Notes\n\nImplement it.", 1)
	writeTask(t, root, "ready", "TF-105-heading.md", strings.Replace(content, "TF-101", "TF-105", 1))
	joined := diagnosticsText(Validate(root, ""))
	if !strings.Contains(joined, "Scope: unsupported heading") {
		t.Fatalf("diagnostics missing unsupported heading:\n%s", joined)
	}
}

func diagnosticsText(diagnostics []Diagnostic) string {
	var joined strings.Builder
	for _, diagnostic := range diagnostics {
		joined.WriteString(diagnostic.Error())
		joined.WriteByte('\n')
	}
	return joined.String()
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

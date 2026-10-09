package verify

import "testing"

func TestClaimFieldsReadsClaimBlock(t *testing.T) {
	data := []byte("# TF-001: Example\n\n## Scope\n\nBranch: not a claim\n\n" +
		"## Claim\n\nOwner: w\nBranch: task/TF-001-impl\n" +
		"Worktree: /tmp/wt/TF-001\nBase commit: 0123456789abcdef0123456789abcdef01234567\n" +
		"Started at: 2026-10-09T10:00:00Z\n")
	got := ClaimFields(data)
	want := map[string]string{
		"Branch":      "task/TF-001-impl",
		"Base commit": "0123456789abcdef0123456789abcdef01234567",
		"Worktree":    "/tmp/wt/TF-001",
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("ClaimFields[%q] = %q, want %q", key, got[key], value)
		}
	}
}

func TestClaimFieldsMissingBlockIsEmpty(t *testing.T) {
	if got := ClaimFields([]byte("# TF-001: Example\n\n## Goal\n\nText\n")); len(got) != 0 {
		t.Fatalf("ClaimFields without a Claim block = %v, want empty", got)
	}
}

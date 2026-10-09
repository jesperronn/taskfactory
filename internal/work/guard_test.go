package work

import (
	"os"
	"strings"
	"testing"
)

// CheckNoCommit fails when a non-test Go file in this package overrides signing
// or names a Git command that writes: work must never stage or commit.
func TestCheckNoCommitOrSigningOverride(t *testing.T) {
	banned := []string{"no-gpg-sign", "gpgsign", `"commit"`, `"add"`, `"push"`, `"reset"`, `"checkout"`, `"stash"`, `"merge"`, `"rebase"`, `"mv"`, `"rm"`, `"restore"`}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, word := range banned {
			if strings.Contains(string(data), word) {
				t.Errorf("%s contains %s", name, word)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no non-test Go files were checked")
	}
}

package requeue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CheckNoSigningOverride fails when any non-test Go file in this package
// contains a signing override. The requeue command must run a plain git commit.
func TestCheckNoSigningOverride(t *testing.T) {
	banned := []string{"no-gpg-sign", "gpgsign"}
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
		data, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, word := range banned {
			if strings.Contains(string(data), word) {
				t.Errorf("%s contains signing override %q", name, word)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no non-test Go files were checked")
	}
}

package newtask

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Add a Thing":        "add-a-thing",
		"  --Hello,  World!": "hello-world",
		"v2.0 release":       "v2-0-release",
		"!!!":                "",
		"Ünï":                "n",
		"a--b__c":            "a-b-c",
	} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
	long := Slug("word word word word word word word word word word word word word word")
	if len(long) > maxSlug || long[len(long)-1] == '-' {
		t.Errorf("long slug %q", long)
	}
}

func TestNextIDAndCreate(t *testing.T) {
	root := t.TempDir()
	if id, err := NextID(root); err != nil || id != "TF-001" {
		t.Fatalf("empty tree: %q, %v", id, err)
	}
	for state, name := range map[string]string{"archive": "TF-009-a.md", "failed": "TF-004-b.md"} {
		dir := filepath.Join(root, "tasks", state)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Create(root, "Hello", Options{})
	if err != nil || res.ID != "TF-010" || res.Rel != "tasks/inbox/TF-010-hello.md" {
		t.Fatalf("Create: %+v, %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(root, "tasks", "inbox", "TF-010-hello.md")); err != nil {
		t.Fatal(err)
	}
	var usage UsageError
	if _, err := Create(root, "???", Options{}); err == nil || !asUsage(err, &usage) {
		t.Fatalf("unusable title error = %v", err)
	}
	if _, err := Create(root, "", Options{}); err == nil || !asUsage(err, &usage) {
		t.Fatalf("empty title error = %v", err)
	}
}

func asUsage(err error, target *UsageError) bool {
	u, ok := err.(UsageError)
	if ok {
		*target = u
	}
	return ok
}

package config

import (
	"strings"
	"testing"
)

func TestBaseBranchDefaultsToMain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeStateConfig(t, root, "protocol_version = 1\n")
	cfg, err := Load(root)
	if err != nil || cfg.Git.BaseBranch != "main" {
		t.Fatalf("Load = %q, %v; want main", cfg.Git.BaseBranch, err)
	}
	if got, err := BaseBranchOf(root); err != nil || got != "main" {
		t.Fatalf("BaseBranchOf = %q, %v", got, err)
	}
	if got, err := BaseBranchOf(t.TempDir()); err != nil || got != "main" {
		t.Fatalf("BaseBranchOf without config = %q, %v", got, err)
	}
}

func TestBaseBranchAccepts(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"master", "trunk", "develop", "release/1.0", "feature_x-1", "v1.2"} {
		root := t.TempDir()
		writeStateConfig(t, root, "[git]\nbase_branch = \""+value+"\"\n")
		cfg, err := Load(root)
		if err != nil || cfg.Git.BaseBranch != value {
			t.Errorf("Load with %q = %q, %v", value, cfg.Git.BaseBranch, err)
		}
		if got, err := BaseBranchOf(root); err != nil || got != value {
			t.Errorf("BaseBranchOf(%q) = %q, %v", value, got, err)
		}
	}
}

func TestBaseBranchRejects(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"", " ", "a b", "-x", "--force", "a..b", "/a", "a/", "a//b", "a.", "a.lock", "a/.b", "a/b.lock",
		".hidden", "a~1", "a^", "a:b", "a?", "a*", "a[", "a\\\\b", "a@{1}", "@", "a\\tb",
	} {
		root := t.TempDir()
		writeStateConfig(t, root, "[git]\nbase_branch = \""+value+"\"\n")
		for name, err := range map[string]error{"Load": loadErr(root), "BaseBranchOf": baseErr(root)} {
			if err == nil || !strings.Contains(err.Error(), "git.base_branch") || !strings.Contains(err.Error(), "config.toml") {
				t.Errorf("%s with %q: err = %v", name, value, err)
			}
		}
	}
}

func TestBaseBranchRejectsWrongType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeStateConfig(t, root, "[git]\nbase_branch = 7\n")
	if _, err := Load(root); err == nil {
		t.Fatal("Load accepted an integer base_branch")
	}
}

func loadErr(root string) error { _, err := Load(root); return err }

func baseErr(root string) error { _, err := BaseBranchOf(root); return err }

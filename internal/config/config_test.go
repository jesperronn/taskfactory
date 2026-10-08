package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    Config
		wantErr string
	}{
		{
			name: "full config",
			content: `protocol_version = 1

[workers]
max_parallel = 4

[git]
use_worktrees = true
worktree_root = ".taskfactory/worktrees"
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = ["go test ./..."]
integration = ["go test ./...", "go vet ./..."]
main = ["go test ./..."]
`,
			want: Config{
				ProtocolVersion: 1,
				Workers:         Workers{MaxParallel: 4},
				Git:             Git{UseWorktrees: true, WorktreeRoot: filepath.Join(".taskfactory", "worktrees"), IntegrationStrategy: "ff-only"},
				Integration:     Integration{StopOnMainFailure: true},
				Verification: Verification{
					Worker: []string{"go test ./..."}, Integration: []string{"go test ./...", "go vet ./..."}, Main: []string{"go test ./..."},
				},
			},
		},
		{
			name:    "defaults",
			content: "",
			want: Config{
				ProtocolVersion: 1,
				Workers:         Workers{MaxParallel: 4},
				Git:             Git{UseWorktrees: true, WorktreeRoot: ".taskfactory/worktrees", IntegrationStrategy: "ff-only"},
				Integration:     Integration{StopOnMainFailure: true},
				Verification:    Verification{Worker: []string{"go test ./..."}, Integration: []string{"go test ./...", "go vet ./..."}},
			},
		},
		{name: "unsupported version", content: "protocol_version = 2\n", wantErr: "protocol_version 2 is unsupported"},
		{name: "wrong type", content: "[git]\nuse_worktrees = \"yes\"\n", wantErr: "git.use_worktrees"},
		{name: "worker limit zero", content: "[workers]\nmax_parallel = 0\n", wantErr: "workers.max_parallel 0 is outside the allowed range 1..4"},
		{name: "worker limit five", content: "[workers]\nmax_parallel = 5\n", wantErr: "workers.max_parallel 5 is outside the allowed range 1..4"},
		{name: "unknown key", content: "future_option = true\n", wantErr: "unknown key future_option"},
		{name: "unknown table", content: "[experimental]\ncache_ttl = 60\n", wantErr: "unknown table [experimental]"},
		{name: "empty unknown table", content: "[experimental]\n", wantErr: "unknown table [experimental]"},
		{name: "worktrees disabled", content: "[git]\nuse_worktrees = false\n", wantErr: "git.use_worktrees must be true"},
		{name: "empty verification", content: "[verification]\nworker = []\n", wantErr: "verification.worker"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, tt.content)
			got, err := Load(root)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %v, want substring %q", err, tt.wantErr)
				}
				if !strings.Contains(err.Error(), filepath.Join(root, ".taskfactory", "config.toml")) {
					t.Fatalf("Load() error %q does not identify config file", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			tt.want.Git.WorktreeRoot = filepath.Join(realPath(t, root), ".taskfactory", "worktrees")
			if got.Git.WorktreeRoot != tt.want.Git.WorktreeRoot {
				t.Fatalf("WorktreeRoot = %q, want %q", got.Git.WorktreeRoot, tt.want.Git.WorktreeRoot)
			}
			got.Git.WorktreeRoot = tt.want.Git.WorktreeRoot
			if !equalConfig(got, tt.want) {
				t.Errorf("Load() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	root := t.TempDir()
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), filepath.Join(root, ".taskfactory", "config.toml")) || !strings.Contains(err.Error(), "taskfactory init") {
		t.Fatalf("Load() error = %v, want actionable missing config error", err)
	}
}

func TestLoadResolvesRelativeWorktreeRootFromProjectRoot(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "[git]\nworktree_root = \"../workers\"\n")
	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(filepath.Dir(realPath(t, root)), "workers"); got.Git.WorktreeRoot != want {
		t.Fatalf("WorktreeRoot = %q, want %q", got.Git.WorktreeRoot, want)
	}
}

func TestLoadCanonicalizesExistingSymlinkAndAllowsAbsoluteRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "worktrees")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	writeConfig(t, root, "[git]\nworktree_root = \""+link+"\"\n")
	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := realPath(t, target); got.Git.WorktreeRoot != want {
		t.Fatalf("WorktreeRoot = %q, want canonical absolute path %q", got.Git.WorktreeRoot, want)
	}
}

func realPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func writeConfig(t *testing.T, root, content string) {
	t.Helper()
	path := filepath.Join(root, ".taskfactory", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func equalConfig(a, b Config) bool {
	if a.ProtocolVersion != b.ProtocolVersion || a.Workers != b.Workers || a.Git != b.Git || a.Integration != b.Integration {
		return false
	}
	return equalStrings(a.Verification.Worker, b.Verification.Worker) && equalStrings(a.Verification.Integration, b.Verification.Integration) && equalStrings(a.Verification.Main, b.Verification.Main)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

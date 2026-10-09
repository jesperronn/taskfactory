package work

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"taskfactory/internal/adapter/common"
)

const (
	testID     = "TF-056"
	testBranch = "task/TF-056-impl"
	testConfig = `protocol_version = 1

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
`
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=Test Worker", "GIT_AUTHOR_EMAIL=worker@example.invalid",
		"GIT_COMMITTER_NAME=Test Worker", "GIT_COMMITTER_EMAIL=worker@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type fixture struct {
	root, worktree, base string
}

// newFixture builds a repository on main with an active claimed task and a
// real worktree. With claim false the task file has no Claim block.
func newFixture(t *testing.T, claim bool) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "config", "commit.gpgsign", "false")
	write(t, filepath.Join(root, ".taskfactory", "config.toml"), testConfig)
	write(t, filepath.Join(root, "docs", "worker-instructions.md"), "# Worker instructions\n")
	write(t, filepath.Join(root, "example.txt"), "base\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "init")
	base := git(t, root, "rev-parse", "refs/heads/main")
	worktree := filepath.Join(root, ".taskfactory", "worktrees", testID)
	git(t, root, "worktree", "add", "-q", worktree, "-b", testBranch, "main")
	task := "# " + testID + ": Example\n\n## Goal\n\nDo it.\n\n## Dependencies\n\nNone\n\n" +
		"## Scope\n\nExample file.\n\n## Constraints\n\nStandard library.\n\n" +
		"## Success criteria\n\n### C1: Passes\n\nCheck: go test ./...\n\n## Verification\n\nRun it.\n"
	if claim {
		task += "\n## Claim\n\nOwner: w\nBranch: " + testBranch + "\nWorktree: " + worktree +
			"\nBase commit: " + base + "\nStarted at: 2026-10-09T10:00:00Z\n"
	}
	write(t, filepath.Join(root, "tasks", "active", testID+"-example.md"), task)
	return fixture{root: root, worktree: worktree, base: base}
}

var fixedNow = func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) }

func intp(n int) *int { return &n }

func okPreflight(context.Context, Request) common.PreflightResult {
	return common.PreflightResult{Checks: []common.PreflightCheck{{Name: "adapter binary omp"}}}
}

// recorder is a fake adapter runner that records its calls.
type recorder struct {
	preflights, runs int
	last             Request
	result           common.Result
}

func (r *recorder) deps(out *strings.Builder) Deps {
	return Deps{
		Preflight: func(ctx context.Context, req Request) common.PreflightResult {
			r.preflights++
			return okPreflight(ctx, req)
		},
		Run: func(_ context.Context, req Request) common.Result {
			r.runs++
			r.last = req
			return r.result
		},
		Now:     fixedNow,
		Environ: func() []string { return nil },
		Stdout:  out,
	}
}

func baseOpts() Options {
	return Options{Adapter: "omp", Model: "omlx/m", Endpoint: DefaultEndpoint, Timeout: time.Minute}
}

func logFiles(t *testing.T, root string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(root, ".taskfactory", "logs", "*", "*.log"))
	return m
}

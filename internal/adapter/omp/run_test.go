package omp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStubRunCreatesFileInTempWorktree(t *testing.T) {
	installStub(t, testModel)
	req := testRequest(t)
	res := Run(context.Background(), req, Options{Dial: okDial})
	if res.State != StateExit {
		t.Fatalf("state = %s, want exit; note=%s output=%s", res.State, res.Note, res.Output)
	}
	if res.ExitCode == nil || *res.ExitCode != 0 {
		t.Fatalf("exit code = %v, want 0", res.ExitCode)
	}
	data, err := os.ReadFile(filepath.Join(req.Worktree, "created.txt"))
	if err != nil {
		t.Fatalf("stub did not create file in worktree: %v", err)
	}
	if strings.TrimSpace(string(data)) != "stub ran" {
		t.Errorf("created.txt = %q", data)
	}
	argv, err := os.ReadFile(filepath.Join(req.Worktree, "argv.txt"))
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if !strings.Contains(string(argv), testModel) {
		t.Errorf("launched argv does not name the explicit model: %s", argv)
	}
}

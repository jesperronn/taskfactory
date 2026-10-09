package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequeueUsageErrorsExitTwoAndHelpExitsZero(t *testing.T) {
	binary := buildCLI(t)
	outside := t.TempDir()

	tests := []struct {
		name     string
		args     []string
		exitCode int
		want     []string
	}{
		{"help", []string{"requeue", "--help"}, 0, []string{"Usage: taskfactory requeue <ID> [--to ready|inbox]", "human decision", "counter", "Exit codes:"}},
		{"short help", []string{"requeue", "TF-001", "-h"}, 0, []string{"Usage: taskfactory requeue"}},
		{"missing ID", []string{"requeue"}, 2, []string{"missing task ID"}},
		{"unknown target", []string{"requeue", "TF-001", "--to", "active"}, 2, []string{"ready or inbox"}},
		{"extra argument", []string{"requeue", "TF-001", "TF-002"}, 2, []string{"extra argument"}},
		{"unknown flag", []string{"requeue", "TF-001", "--force"}, 2, []string{"unknown argument"}},
		{"bad ID", []string{"requeue", "nope"}, 2, []string{"not a valid task ID"}},
		{"outside a project", []string{"requeue", "TF-001"}, 1, []string{"not inside a Git working tree"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runCLI(t, binary, outside, tt.args...)
			if got := processExitCode(err); got != tt.exitCode {
				t.Fatalf("exit = %d, want %d; output: %s", got, tt.exitCode, out)
			}
			for _, want := range tt.want {
				if !strings.Contains(string(out), want) {
					t.Errorf("output %q lacks %q", out, want)
				}
			}
		})
	}

	top, err := runCLI(t, binary, outside, "--help")
	if err != nil || !strings.Contains(string(top), "requeue <ID> [--to ready|inbox]") {
		t.Errorf("top-level help does not list requeue: %v %s", err, top)
	}
}

func TestRequeueHelpListsClaimedFailedTasksInsideProject(t *testing.T) {
	binary := buildCLI(t)
	project := initGitProject(t)
	failedDir := filepath.Join(project, "tasks", "failed")
	if err := os.MkdirAll(failedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, binary, project, "requeue", "--help")
	if err != nil || !strings.Contains(string(out), "Claimed failed tasks in this project: none.") {
		t.Fatalf("empty project help: %v\n%s", err, out)
	}
	for name, body := range map[string]string{
		"TF-007-claimed.md": "# TF-007: x\n\n## Claim\n\nOwner: a\n",
		"TF-008-plain.md":   "# TF-008: x\n",
	} {
		if err := os.WriteFile(filepath.Join(failedDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err = runCLI(t, binary, project, "requeue", "-h")
	if err != nil {
		t.Fatalf("help failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "human decision needed): TF-007\n") || strings.Contains(string(out), "TF-008") {
		t.Errorf("help does not list only TF-007: %s", out)
	}
}

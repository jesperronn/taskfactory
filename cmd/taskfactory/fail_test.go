package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestFailUsageErrorsExitTwoAndHelpExitsZero(t *testing.T) {
	binary := buildCLI(t)
	outsideCheckout := t.TempDir()

	tests := []struct {
		name       string
		args       []string
		exitCode   int
		wantOutput []string
	}{
		{name: "help", args: []string{"fail", "--help"}, exitCode: 0, wantOutput: []string{"Usage: taskfactory fail <ID> --outcome", "Exit codes:", "--reason <text>"}},
		{name: "short help", args: []string{"fail", "TF-001", "-h"}, exitCode: 0, wantOutput: []string{"Usage: taskfactory fail"}},
		{name: "missing ID and outcome", args: []string{"fail"}, exitCode: 2, wantOutput: []string{"missing task ID"}},
		{name: "missing outcome", args: []string{"fail", "TF-001"}, exitCode: 2, wantOutput: []string{"missing --outcome"}},
		{name: "unknown outcome", args: []string{"fail", "TF-001", "--outcome", "DONE"}, exitCode: 2, wantOutput: []string{"FAILED or BLOCKED"}},
		{name: "empty reason", args: []string{"fail", "TF-001", "--outcome", "BLOCKED", "--reason", ""}, exitCode: 2, wantOutput: []string{"non-empty"}},
		{name: "extra argument", args: []string{"fail", "TF-001", "TF-002", "--outcome", "FAILED"}, exitCode: 2, wantOutput: []string{"extra argument"}},
		{name: "unknown flag", args: []string{"fail", "TF-001", "--outcome", "FAILED", "--force"}, exitCode: 2, wantOutput: []string{"unknown argument"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binary, tt.args...)
			cmd.Dir = outsideCheckout
			output, err := cmd.CombinedOutput()
			if got := processExitCode(err); got != tt.exitCode {
				t.Fatalf("exit code = %d, want %d; output: %s", got, tt.exitCode, output)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(string(output), want) {
					t.Errorf("output %q does not contain %q", output, want)
				}
			}
		})
	}
}

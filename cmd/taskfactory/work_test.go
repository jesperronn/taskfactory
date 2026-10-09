package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestWorkUsageErrorsExitTwoAndHelpExitsZero(t *testing.T) {
	binary := buildCLI(t)
	outsideCheckout := t.TempDir()

	tests := []struct {
		name       string
		args       []string
		exitCode   int
		wantOutput []string
	}{
		{"help", []string{"work", "--help"}, 0, []string{"Usage: taskfactory work <ID> --adapter", "Exit codes:", "--haiku-model <id>", "--endpoint <host:port>"}},
		{"short help", []string{"work", "-h"}, 0, []string{"Usage: taskfactory work"}},
		{"help after flags", []string{"work", "TF-001", "--adapter", "omp", "-h"}, 0, []string{"Usage: taskfactory work"}},
		{"no arguments", []string{"work"}, 2, []string{"missing task ID"}},
		{"missing adapter", []string{"work", "TF-001", "--model", "m"}, 2, []string{"missing --adapter"}},
		{"missing model", []string{"work", "TF-001", "--adapter", "omp"}, 2, []string{"missing --model"}},
		{"two adapter flags", []string{"work", "TF-001", "--adapter", "omp", "--adapter", "pi", "--model", "m"}, 2, []string{"exactly one --adapter"}},
		{"unknown adapter", []string{"work", "TF-001", "--adapter", "gptme", "--model", "m"}, 2, []string{"unknown adapter"}},
		{"claude without haiku model", []string{"work", "TF-001", "--adapter", "claude", "--model", "m"}, 2, []string{"requires --haiku-model"}},
		{"timeout too short", []string{"work", "TF-001", "--adapter", "omp", "--model", "m", "--timeout", "1ms"}, 2, []string{"at least 1s"}},
		{"unknown flag", []string{"work", "TF-001", "--adapter", "omp", "--model", "m", "--force"}, 2, []string{"unknown argument"}},
		{"extra argument", []string{"work", "TF-001", "TF-002", "--adapter", "omp", "--model", "m"}, 2, []string{"extra argument"}},
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

func TestWorkRefusesWithoutActiveTaskBeforeLaunchExitsOne(t *testing.T) {
	binary := buildCLI(t)
	root := initGitProject(t)
	cmd := exec.Command(binary, "work", "TF-001", "--adapter", "omp", "--model", "m")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if got := processExitCode(err); got != 1 || !strings.Contains(string(output), "tasks/active") {
		t.Fatalf("exit=%d output=%s", got, output)
	}
}

func TestWorkIsListedInUsageAndHasHelp(t *testing.T) {
	if _, ok := commandHelps["work"]; !ok {
		t.Fatal("work missing from commandHelps")
	}
	found := false
	for _, c := range commandSummaries {
		found = found || c.name == "work"
	}
	if !found {
		t.Fatal("work missing from commandSummaries")
	}
}

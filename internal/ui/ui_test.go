package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func fakeEnv(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func terminalFile(t *testing.T) *os.File {
	t.Helper()
	// /dev/null is a character device, standing in for a terminal.
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func pipeFile(t *testing.T) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
	return r
}

func TestEnabled(t *testing.T) {
	regular := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileHandle, err := os.Open(regular)
	if err != nil {
		t.Fatal(err)
	}
	defer fileHandle.Close()

	tests := []struct {
		name string
		file *os.File
		env  map[string]string
		want bool
	}{
		{name: "terminal with no env", file: terminalFile(t), env: nil, want: true},
		{name: "pipe is off", file: pipeFile(t), env: nil, want: false},
		{name: "regular file is off", file: fileHandle, env: nil, want: false},
		{name: "NO_COLOR disables terminal", file: terminalFile(t), env: map[string]string{"NO_COLOR": "1"}, want: false},
		{name: "empty NO_COLOR is ignored", file: terminalFile(t), env: map[string]string{"NO_COLOR": ""}, want: true},
		{name: "TERM=dumb disables terminal", file: terminalFile(t), env: map[string]string{"TERM": "dumb"}, want: false},
		{name: "TERM=xterm keeps terminal", file: terminalFile(t), env: map[string]string{"TERM": "xterm-256color"}, want: true},
		{name: "FORCE_COLOR forces pipe on", file: pipeFile(t), env: map[string]string{"FORCE_COLOR": "1"}, want: true},
		{name: "FORCE_COLOR forces regular file on", file: fileHandle, env: map[string]string{"FORCE_COLOR": "1"}, want: true},
		{name: "NO_COLOR beats FORCE_COLOR", file: pipeFile(t), env: map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1"}, want: false},
		{name: "TERM=dumb beats FORCE_COLOR", file: pipeFile(t), env: map[string]string{"TERM": "dumb", "FORCE_COLOR": "1"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Enabled(tt.file, fakeEnv(tt.env)); got != tt.want {
				t.Errorf("Enabled() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestPainterOffLeavesTextUnchanged(t *testing.T) {
	var p Painter
	cases := map[string]string{
		p.Bold("Usage:"): "Usage:",
		p.Dim("<ID>"):    "<ID>",
		p.Red("error"):   "error",
		p.Green("valid"): "valid",
		p.Yellow("warn"): "warn",
		p.Cyan("status"): "status",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("painter off: got %q, want %q", got, want)
		}
	}
}

func TestPainterOnEmitsSGR(t *testing.T) {
	p := Painter{on: true}
	tests := map[string]string{
		p.Bold("Usage:"):   "\x1b[1mUsage:\x1b[0m",
		p.Dim("<ID>"):      "\x1b[2m<ID>\x1b[0m",
		p.Red("error"):     "\x1b[31merror\x1b[0m",
		p.Green("valid"):   "\x1b[32mvalid\x1b[0m",
		p.Yellow("warn"):   "\x1b[33mwarn\x1b[0m",
		p.Cyan("validate"): "\x1b[36mvalidate\x1b[0m",
	}
	for got, want := range tests {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

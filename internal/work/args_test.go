package work

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseArgsAcceptsValidForms(t *testing.T) {
	id, o, err := ParseArgs([]string{"TF-056", "--adapter=pi", "--model", "m", "--timeout", "30s", "--endpoint", "127.0.0.1:9"})
	if err != nil || id != "TF-056" || o.Adapter != "pi" || o.Model != "m" || o.Timeout != 30*time.Second || o.Endpoint != "127.0.0.1:9" {
		t.Fatalf("id=%s o=%+v err=%v", id, o, err)
	}
	_, o, err = ParseArgs([]string{"--adapter", "claude", "--model", "a", "--haiku-model", "b", "TF-056"})
	if err != nil || o.HaikuModel != "b" || o.Timeout != DefaultTimeout || o.Endpoint != DefaultEndpoint {
		t.Fatalf("o=%+v err=%v", o, err)
	}
}

func TestParseArgsUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no args", nil, "missing task ID"},
		{"missing adapter", []string{"TF-056", "--model", "m"}, "missing --adapter"},
		{"missing model", []string{"TF-056", "--adapter", "omp"}, "missing --model"},
		{"two adapter flags", []string{"TF-056", "--adapter", "omp", "--adapter", "pi", "--model", "m"}, "exactly one --adapter"},
		{"repeated same adapter", []string{"TF-056", "--adapter=omp", "--adapter=omp", "--model", "m"}, "exactly one --adapter"},
		{"unknown adapter", []string{"TF-056", "--adapter", "gpt", "--model", "m"}, "unknown adapter"},
		{"model whitespace", []string{"TF-056", "--adapter", "omp", "--model", "a b"}, "whitespace"},
		{"short timeout", []string{"TF-056", "--adapter", "omp", "--model", "m", "--timeout", "500ms"}, "at least 1s"},
		{"bad timeout", []string{"TF-056", "--adapter", "omp", "--model", "m", "--timeout", "soon"}, "not a duration"},
		{"unknown flag", []string{"TF-056", "--adapter", "omp", "--model", "m", "--force"}, "unknown argument"},
		{"extra argument", []string{"TF-056", "TF-057", "--adapter", "omp", "--model", "m"}, "extra argument"},
		{"flag without value", []string{"TF-056", "--adapter"}, "requires a value"},
		{"claude without haiku", []string{"TF-056", "--adapter", "claude", "--model", "m"}, "requires --haiku-model"},
		{"haiku for omp", []string{"TF-056", "--adapter", "omp", "--model", "m", "--haiku-model", "h"}, "only to --adapter claude"},
		{"repeated model", []string{"TF-056", "--adapter", "omp", "--model", "m", "--model", "n"}, "only once"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseArgs(tt.args)
			var usage UsageError
			if !errors.As(err, &usage) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want usage error containing %q", err, tt.want)
			}
		})
	}
}

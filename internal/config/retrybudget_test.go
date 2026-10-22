package config

import (
	"strings"
	"testing"
)

func TestRetryBudgetKeyParses(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
		wantErr string
	}{
		{name: "unset takes default", content: "", want: DefaultRetryBudget},
		{name: "set", content: "retry_budget = 3\n", want: 3},
		{name: "one attempt", content: "retry_budget = 1\n", want: 1},
		{name: "zero is invalid", content: "retry_budget = 0\n", wantErr: "retry_budget 0 must be at least 1"},
		{name: "negative is invalid", content: "retry_budget = -2\n", wantErr: "retry_budget -2 must be at least 1"},
		{name: "wrong type", content: "retry_budget = \"3\"\n", wantErr: "retry_budget"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, tt.content)

			got, err := Load(root)
			gotOf, errOf := RetryBudgetOf(root)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %v, want substring %q", err, tt.wantErr)
				}
				if errOf == nil || !strings.Contains(errOf.Error(), tt.wantErr) {
					t.Fatalf("RetryBudgetOf() error = %v, want substring %q", errOf, tt.wantErr)
				}
				return
			}
			if err != nil || errOf != nil {
				t.Fatalf("Load() error = %v, RetryBudgetOf() error = %v", err, errOf)
			}
			if got.RetryBudget != tt.want || gotOf != tt.want {
				t.Fatalf("RetryBudget = %d, RetryBudgetOf = %d, want %d", got.RetryBudget, gotOf, tt.want)
			}
		})
	}
}

package work

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taskfactory/internal/adapter/common"
)

// TestBackToBackRunsInOneSecondBothSucceed runs work twice with the same
// injected clock, so both runs start in the same second. Both must exit 0,
// both logs must exist, and the printed log paths must be the files used.
func TestBackToBackRunsInOneSecondBothSucceed(t *testing.T) {
	f := newFixture(t, true)
	rec := &recorder{result: common.Result{State: common.StateExit, ExitCode: intp(0), Output: "ok"}}
	var printed []string
	for i := 0; i < 2; i++ {
		var out strings.Builder
		code, err := Execute(context.Background(), f.root, testID, baseOpts(), rec.deps(&out))
		if code != 0 || err != nil {
			t.Fatalf("run %d: code=%d err=%v\n%s", i+1, code, err, out.String())
		}
		line := strings.SplitN(out.String(), "\n", 2)[0]
		if !strings.HasPrefix(line, "log: ") {
			t.Fatalf("run %d first line = %q, want a log: line", i+1, line)
		}
		printed = append(printed, strings.TrimPrefix(line, "log: "))
	}
	if rec.runs != 2 {
		t.Fatalf("adapter ran %d times, want 2", rec.runs)
	}
	if printed[0] == printed[1] {
		t.Fatalf("both runs printed the same log path %s", printed[0])
	}
	dir := filepath.Join(f.root, ".taskfactory", "logs", testID)
	for _, p := range printed {
		if filepath.Dir(p) != dir {
			t.Fatalf("printed log %s is outside %s", p, dir)
		}
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("printed log does not exist: %v", err)
		}
	}
	if logs := logFiles(t, f.root); len(logs) != 2 {
		t.Fatalf("logs = %v, want 2 files", logs)
	}
}

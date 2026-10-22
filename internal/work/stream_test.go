package work

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"taskfactory/internal/adapter/common"
)

// syncBuffer is a terminal stand-in that is safe to read while Execute writes.
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// TestStreamsWorkerOutputBeforeExit runs a fake worker that prints one line and
// then waits for a release file. The line must reach the terminal and the log
// while the worker is still running, not after it exits.
func TestStreamsWorkerOutputBeforeExit(t *testing.T) {
	f := newFixture(t, true)
	dir := t.TempDir()
	release := filepath.Join(dir, "release")
	script := filepath.Join(dir, "worker.sh")
	body := "#!/bin/sh\necho first line from worker\nwhile [ ! -f '" + release + "' ]; do sleep 0.05; done\necho last line\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(ctx context.Context, req Request) common.Result {
		return common.Launch(ctx, common.LaunchSpec{Binary: script, Dir: req.Worktree, Timeout: 30 * time.Second, Stream: req.Stream})
	}
	stdout := &syncBuffer{}
	type outcome struct {
		code int
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		code, err := Execute(context.Background(), f.root, testID, baseOpts(), Deps{
			Preflight: okPreflight, Run: run, Now: fixedNow,
			Environ: func() []string { return nil }, Stdout: stdout,
		})
		done <- outcome{code, err}
	}()

	const line = "first line from worker"
	seenTerminal, seenLog := false, false
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if strings.Contains(stdout.String(), line) {
			seenTerminal = true
			for _, p := range logFiles(t, f.root) {
				if b, err := os.ReadFile(p); err == nil && strings.Contains(string(b), line) {
					seenLog = true
				}
			}
			break
		}
	}
	// Release the worker whatever happened, so a failure never hangs the test.
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var got outcome
	select {
	case got = <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("Execute did not return after release")
	}
	if !seenTerminal {
		t.Fatalf("worker output did not reach the terminal before exit; terminal:\n%s", stdout.String())
	}
	if !seenLog {
		t.Error("worker output did not reach the log before exit")
	}
	if got.code != 0 || got.err != nil {
		t.Fatalf("code=%d err=%v, want 0", got.code, got.err)
	}

	logs := logFiles(t, f.root)
	if len(logs) != 1 {
		t.Fatalf("logs = %v, want one", logs)
	}
	b, err := os.ReadFile(logs[0])
	if err != nil {
		t.Fatal(err)
	}
	log := string(b)
	sep := "----- adapter output -----\n"
	i := strings.Index(log, sep)
	if i < 0 || !strings.Contains(log[:i], "State: ") || !strings.Contains(log[:i], "Exit code: 0\n") {
		t.Fatalf("log layout wrong:\n%s", log)
	}
	if !strings.HasSuffix(log, sep+"first line from worker\nlast line\n\n") {
		t.Errorf("log body wrong:\n%s", log)
	}
}

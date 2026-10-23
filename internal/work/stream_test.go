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

// TestStreamRedactsMultiLineSecret runs a fake worker that prints a credential
// value spanning two lines in several chunks. Neither line of the value may
// reach the terminal or the log, before or after the run exits, while the
// ordinary line before it still arrives before the run exits. Failure messages
// name no part of the value.
func TestStreamRedactsMultiLineSecret(t *testing.T) {
	const (
		firstLine  = "tf113-fake-first-line"
		secondLine = "tf113-fake-second-line"
		ordinary   = "tf113 ordinary line"
		lastLine   = "tf113 last line"
	)
	f := newFixture(t, true)
	dir := t.TempDir()
	release := filepath.Join(dir, "release")
	script := filepath.Join(dir, "worker.sh")
	body := "#!/bin/sh\n" +
		"printf '%s\\n' '" + ordinary + "'\nsleep 0.1\n" +
		"printf '%s\\n' '" + firstLine + "'\nsleep 0.1\n" +
		"printf '%s' 'tf113-fake-second'\nsleep 0.1\n" +
		"printf '%s\\n' '-line'\n" +
		"while [ ! -f '" + release + "' ]; do sleep 0.05; done\n" +
		"printf '%s\\n' '" + lastLine + "'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(ctx context.Context, req Request) common.Result {
		return common.Launch(ctx, common.LaunchSpec{Binary: script, Dir: req.Worktree, Timeout: 30 * time.Second, Stream: req.Stream})
	}
	environ := func() []string {
		return []string{"TF113_FAKE_TOKEN=" + firstLine + "\n" + secondLine}
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
			Environ: environ, Stdout: stdout,
		})
		done <- outcome{code, err}
	}()

	// Wait until the ordinary line and the redaction marker have reached the
	// terminal, which is still before the worker exits.
	seenBefore := false
	var before, beforeLog string
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		s := stdout.String()
		if strings.Contains(s, ordinary) && strings.Contains(s, "[redacted]") {
			seenBefore = true
			before = s
			for _, p := range logFiles(t, f.root) {
				if b, err := os.ReadFile(p); err == nil {
					beforeLog += string(b)
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
	for _, leak := range []string{"tf113-fake-first", "tf113-fake-second"} {
		if strings.Contains(before, leak) {
			t.Error("part of a multi-line secret reached the terminal before exit")
		}
		if strings.Contains(beforeLog, leak) {
			t.Error("part of a multi-line secret reached the log before exit")
		}
	}
	if !seenBefore {
		t.Fatal("ordinary output or the redaction marker did not reach the terminal before exit")
	}
	if got.code != 0 || got.err != nil {
		t.Fatalf("code=%d err=%v, want 0", got.code, got.err)
	}

	after := stdout.String()
	logs := logFiles(t, f.root)
	if len(logs) != 1 {
		t.Fatalf("logs = %v, want one", logs)
	}
	b, err := os.ReadFile(logs[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"tf113-fake-first", "tf113-fake-second"} {
		if strings.Contains(after, leak) {
			t.Error("part of a multi-line secret reached the terminal")
		}
		if strings.Contains(string(b), leak) {
			t.Error("part of a multi-line secret reached the log")
		}
	}
	if !strings.Contains(after, lastLine) || !strings.Contains(string(b), lastLine) {
		t.Error("output after the secret did not arrive")
	}
}

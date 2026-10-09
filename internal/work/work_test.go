package work

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"taskfactory/internal/adapter/common"
)

func TestPreflightRefusalExitsOneAndChangesNothing(t *testing.T) {
	f := newFixture(t, true)
	statusBefore := git(t, f.root, "status", "--short")
	var out strings.Builder
	rec := &recorder{}
	deps := rec.deps(&out)
	deps.Preflight = func(context.Context, Request) common.PreflightResult {
		return common.PreflightResult{Checks: []common.PreflightCheck{
			{Name: "adapter binary omp", Err: os.ErrNotExist},
			{Name: "endpoint 127.0.0.1:8000", Err: context.DeadlineExceeded},
			{Name: "model", Err: nil},
		}}
	}
	code, err := Execute(context.Background(), f.root, testID, baseOpts(), deps)
	if code != 1 || err == nil {
		t.Fatalf("code=%d err=%v, want 1 and an error", code, err)
	}
	for _, want := range []string{"adapter binary omp", "endpoint 127.0.0.1:8000"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks failed check %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "failed: model") {
		t.Errorf("passing check listed as failed: %v", err)
	}
	if rec.runs != 0 {
		t.Fatalf("adapter ran %d times after a refused preflight", rec.runs)
	}
	if logs := logFiles(t, f.root); len(logs) != 0 {
		t.Fatalf("log written despite refusal: %v", logs)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".taskfactory", "logs")); !os.IsNotExist(err) {
		t.Errorf("logs directory exists after refusal: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout not empty on refusal: %q", out.String())
	}
	if after := git(t, f.root, "status", "--short"); after != statusBefore {
		t.Errorf("status changed:\n%s\n->\n%s", statusBefore, after)
	}
}

func TestRefusesTaskThatIsNotActiveAndClaimed(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, f fixture)
		id    string
		want  string
	}{
		{"no task at all", func(t *testing.T, f fixture) {}, "TF-099", "not in tasks/active"},
		{"task only in ready", func(t *testing.T, f fixture) {
			if err := os.Rename(filepath.Join(f.root, "tasks", "active", testID+"-example.md"), filepath.Join(f.root, "tasks", "ready-"+testID+".md")); err != nil {
				t.Fatal(err)
			}
		}, testID, "not in tasks/active"},
		{"missing worktree", func(t *testing.T, f fixture) {
			git(t, f.root, "worktree", "remove", "--force", f.worktree)
		}, testID, "worktree"},
		{"invalid id", func(t *testing.T, f fixture) {}, "TF-1", "not a valid task ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, true)
			tt.setup(t, f)
			var out strings.Builder
			rec := &recorder{}
			code, err := Execute(context.Background(), f.root, tt.id, baseOpts(), rec.deps(&out))
			if code != 1 || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("code=%d err=%v, want 1 containing %q", code, err, tt.want)
			}
			if rec.preflights+rec.runs != 0 || len(logFiles(t, f.root)) != 0 {
				t.Fatalf("something launched or logged: preflights=%d runs=%d", rec.preflights, rec.runs)
			}
		})
	}
	t.Run("unclaimed active task", func(t *testing.T) {
		f := newFixture(t, false)
		var out strings.Builder
		rec := &recorder{}
		code, err := Execute(context.Background(), f.root, testID, baseOpts(), rec.deps(&out))
		if code != 1 || err == nil || !strings.Contains(err.Error(), "Claim") {
			t.Fatalf("code=%d err=%v, want 1 mentioning the Claim block", code, err)
		}
		if rec.preflights+rec.runs != 0 || len(logFiles(t, f.root)) != 0 {
			t.Fatal("launched or logged for an unclaimed task")
		}
	})
}

func TestExitZeroPrintsVerifyStepAndRunsNothingElse(t *testing.T) {
	f := newFixture(t, true)
	headBefore := git(t, f.root, "rev-parse", "HEAD")
	statusBefore := statusWithoutLogs(t, f.root)
	var out strings.Builder
	rec := &recorder{result: common.Result{State: common.StateExit, ExitCode: intp(0), Output: "done"}}
	code, err := Execute(context.Background(), f.root, testID, baseOpts(), rec.deps(&out))
	if code != 0 || err != nil {
		t.Fatalf("code=%d err=%v, want 0", code, err)
	}
	if !strings.Contains(out.String(), "next: taskfactory verify "+testID+"\n") {
		t.Errorf("output lacks verify step:\n%s", out.String())
	}
	if strings.Contains(out.String(), "fail") || strings.Contains(out.String(), "integrate") {
		t.Errorf("output mentions fail or integrate:\n%s", out.String())
	}
	if rec.preflights != 1 || rec.runs != 1 {
		t.Errorf("preflights=%d runs=%d, want 1 and 1", rec.preflights, rec.runs)
	}
	if rec.last.Worktree != f.worktree || rec.last.Model != "omlx/m" || rec.last.TaskID != testID || rec.last.Timeout != time.Minute {
		t.Errorf("request = %+v", rec.last)
	}
	if !strings.Contains(rec.last.Prompt, "TaskFactory worker prompt") {
		t.Error("prompt was not built with workprompt.Build")
	}
	if strings.Contains(out.String(), "worker commit") {
		t.Errorf("reported a commit although head did not move:\n%s", out.String())
	}
	assertNothingStaged(t, f, headBefore, statusBefore)
}

func TestExitZeroReportsWorkerCommitOnlyWhenHeadMoved(t *testing.T) {
	f := newFixture(t, true)
	var out strings.Builder
	rec := &recorder{result: common.Result{State: common.StateExit, ExitCode: intp(0)}}
	deps := rec.deps(&out)
	deps.Run = func(_ context.Context, req Request) common.Result {
		write(t, filepath.Join(req.Worktree, "example.txt"), "changed\n")
		git(t, req.Worktree, "add", "-A")
		git(t, req.Worktree, "commit", "-q", "-m", "worker change")
		return rec.result
	}
	code, err := Execute(context.Background(), f.root, testID, baseOpts(), deps)
	if code != 0 || err != nil || !strings.Contains(out.String(), "worker commit: ") || !strings.Contains(out.String(), "worker change") {
		t.Fatalf("code=%d err=%v out=%s", code, err, out.String())
	}
}

func TestNonzeroExitPrintsLogPathAndExitsOne(t *testing.T) {
	f := newFixture(t, true)
	var out strings.Builder
	rec := &recorder{result: common.Result{State: common.StateExit, ExitCode: intp(3), Output: "oops"}}
	code, err := Execute(context.Background(), f.root, testID, baseOpts(), rec.deps(&out))
	logs := logFiles(t, f.root)
	if code != 1 || err != nil || len(logs) != 1 || strings.Count(out.String(), logs[0]) < 2 {
		t.Fatalf("code=%d err=%v logs=%v out=%s", code, err, logs, out.String())
	}
	if !strings.Contains(out.String(), "exit code 3") || strings.Contains(out.String(), "suggestion") {
		t.Errorf("unexpected output:\n%s", out.String())
	}
}

func TestSuggestsFailForBlockedAndStalledWithoutRunningIt(t *testing.T) {
	for _, state := range []common.State{common.StateBlocked, common.StateStalled} {
		t.Run(string(state), func(t *testing.T) {
			f := newFixture(t, true)
			headBefore := git(t, f.root, "rev-parse", "HEAD")
			statusBefore := statusWithoutLogs(t, f.root)
			var out strings.Builder
			rec := &recorder{result: common.Result{State: state, Note: "timeout exceeded;\nsaid \"hi\""}}
			code, err := Execute(context.Background(), f.root, testID, baseOpts(), rec.deps(&out))
			logs := logFiles(t, f.root)
			if code != 1 || err != nil || len(logs) != 1 {
				t.Fatalf("code=%d err=%v logs=%v", code, err, logs)
			}
			want := `suggestion, not run: taskfactory fail TF-056 --outcome BLOCKED --reason "timeout exceeded; said \"hi\""`
			if !strings.Contains(out.String(), want) || !strings.Contains(out.String(), logs[0]) {
				t.Errorf("output lacks suggestion or log path:\n%s", out.String())
			}
			if _, err := os.Stat(filepath.Join(f.root, "tasks", "failed")); !os.IsNotExist(err) {
				t.Error("tasks/failed exists: fail was run")
			}
			if _, err := os.Stat(filepath.Join(f.root, "tasks", "active", testID+"-example.md")); err != nil {
				t.Errorf("active task moved: %v", err)
			}
			assertNothingStaged(t, f, headBefore, statusBefore)
		})
	}
}

func TestLogIsWrittenUnderTaskfactoryLogsAndNothingIsStaged(t *testing.T) {
	f := newFixture(t, true)
	headBefore := git(t, f.root, "rev-parse", "HEAD")
	statusBefore := statusWithoutLogs(t, f.root)
	var out strings.Builder
	rec := &recorder{result: common.Result{State: common.StateExit, ExitCode: intp(0), Output: "worker said hello", Note: "a note"}}
	if code, err := Execute(context.Background(), f.root, testID, baseOpts(), rec.deps(&out)); code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	want := filepath.Join(f.root, ".taskfactory", "logs", testID, "omp-20261009T120000Z.log")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"Task: TF-056", "Adapter: omp", "Model: omlx/m", "worker said hello", "Exit code: 0", "Note: a note"} {
		if !strings.Contains(string(data), s) {
			t.Errorf("log lacks %q:\n%s", s, data)
		}
	}
	if !strings.HasPrefix(out.String(), "log: "+want+"\n") {
		t.Errorf("log path not printed first:\n%s", out.String())
	}
	assertNothingStaged(t, f, headBefore, statusBefore)
	if got := git(t, f.root, "status", "--short", "--", ".taskfactory/logs"); !strings.Contains(got, "?? ") {
		t.Errorf("log should be untracked, status: %q", got)
	}
}

func TestLogRefusesToOverwriteExistingLog(t *testing.T) {
	f := newFixture(t, true)
	write(t, filepath.Join(f.root, ".taskfactory", "logs", testID, "omp-20261009T120000Z.log"), "old")
	var out strings.Builder
	rec := &recorder{}
	code, err := Execute(context.Background(), f.root, testID, baseOpts(), rec.deps(&out))
	if code != 1 || err == nil || rec.runs != 0 {
		t.Fatalf("code=%d err=%v runs=%d", code, err, rec.runs)
	}
}

func TestNoCredentialReachesLogOrOutput(t *testing.T) {
	f := newFixture(t, true)
	const secret = "sk-live-0123456789abcdef"
	var out strings.Builder
	rec := &recorder{result: common.Result{State: common.StateStalled, ExitCode: nil,
		Output: "echo " + secret, Note: "token " + secret}}
	deps := rec.deps(&out)
	deps.Environ = func() []string { return []string{"ANTHROPIC_AUTH_TOKEN=" + secret, "HOME=/h"} }
	if code, err := Execute(context.Background(), f.root, testID, baseOpts(), deps); code != 1 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	data, err := os.ReadFile(logFiles(t, f.root)[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) || strings.Contains(out.String(), secret) {
		t.Fatalf("credential leaked:\nlog=%s\nout=%s", data, out.String())
	}
	if !strings.Contains(string(data), "[redacted]") {
		t.Error("expected a redaction marker in the log")
	}
	// A refused preflight must not echo it either.
	deps.Preflight = func(context.Context, Request) common.PreflightResult {
		return common.PreflightResult{Checks: []common.PreflightCheck{{Name: "c", Err: os.ErrInvalid}}}
	}
	f2 := newFixture(t, true)
	_, err = Execute(context.Background(), f2.root, testID, baseOpts(), deps)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("preflight error = %v", err)
	}
}

func TestWorkNeverRunsLifecycleCommands(t *testing.T) {
	f := newFixture(t, true)
	var out strings.Builder
	rec := &recorder{result: common.Result{State: common.StateExit, ExitCode: intp(0)}}
	headBefore := git(t, f.root, "rev-parse", "HEAD")
	statusBefore := statusWithoutLogs(t, f.root)
	if _, err := Execute(context.Background(), f.root, testID, baseOpts(), rec.deps(&out)); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"tasks/failed", "tasks/archive", ".taskfactory/evidence", ".taskfactory/integration-evidence"} {
		if _, err := os.Stat(filepath.Join(f.root, dir)); !os.IsNotExist(err) {
			t.Errorf("%s exists after work (stat err %v)", dir, err)
		}
	}
	assertNothingStaged(t, f, headBefore, statusBefore)
}

func statusWithoutLogs(t *testing.T, root string) string {
	t.Helper()
	var keep []string
	for _, line := range strings.Split(git(t, root, "status", "--short", "-uall"), "\n") {
		if !strings.Contains(line, ".taskfactory/logs/") {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

func assertNothingStaged(t *testing.T, f fixture, headBefore, statusBefore string) {
	t.Helper()
	if head := git(t, f.root, "rev-parse", "HEAD"); head != headBefore {
		t.Errorf("HEAD moved %s -> %s", headBefore, head)
	}
	if wt := git(t, f.worktree, "rev-parse", "HEAD"); wt != f.base {
		t.Errorf("worktree HEAD moved to %s", wt)
	}
	if staged := git(t, f.root, "diff", "--cached", "--name-only"); staged != "" {
		t.Errorf("staged paths: %s", staged)
	}
	if status := statusWithoutLogs(t, f.root); status != statusBefore {
		t.Errorf("status changed:\n%s\n->\n%s", statusBefore, status)
	}
}

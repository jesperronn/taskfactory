package work

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"taskfactory/internal/adapter/claude"
	"taskfactory/internal/adapter/common"
	"taskfactory/internal/adapter/omp"
	"taskfactory/internal/adapter/pi"
	"taskfactory/internal/workprompt"
)

// Request is one adapter launch, the union of the three adapter requests.
type Request struct {
	Adapter    string
	TaskID     string
	Worktree   string
	Model      string
	HaikuModel string
	Endpoint   string
	Timeout    time.Duration
	Prompt     string
}

// Deps holds everything Execute touches outside the filesystem, so tests can
// run it with no model, network or binary. The zero value of each field means
// the real implementation.
type Deps struct {
	// Preflight runs the adapter preflight. Nil runs the real adapter's.
	Preflight func(ctx context.Context, req Request) common.PreflightResult
	// Run launches the adapter. Nil runs the real adapter's.
	Run func(ctx context.Context, req Request) common.Result
	// Now returns the run start time. Nil means time.Now.
	Now func() time.Time
	// Environ returns the environment used only to redact credential values
	// from output. Nil means os.Environ.
	Environ func() []string
	// Stdout receives the progress and next-step lines. Nil means os.Stdout.
	Stdout io.Writer
}

// Execute runs one worker for task id in the project at root. The returned
// code is the process exit code: 0 the worker exited 0, 1 refused, blocked,
// stalled or the worker failed. A non-nil error is a refusal or failure the
// caller prints; it always comes with code 1. Usage errors come from
// ParseArgs, not from here.
func Execute(ctx context.Context, root, id string, opts Options, deps Deps) (int, error) {
	out := deps.Stdout
	if out == nil {
		out = os.Stdout
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	environ := deps.Environ
	if environ == nil {
		environ = os.Environ
	}
	preflight, run := deps.Preflight, deps.Run
	if preflight == nil {
		preflight = realPreflight
	}
	if run == nil {
		run = realRun
	}

	prompt, err := workprompt.Build(root, id)
	if err != nil {
		return 1, err
	}
	worktree, err := filepath.EvalSymlinks(prompt.Worktree)
	if err != nil {
		return 1, fmt.Errorf("refusing to work on %s: worktree %s: %w", id, prompt.Worktree, err)
	}
	req := Request{
		Adapter: opts.Adapter, TaskID: id, Worktree: worktree, Model: opts.Model,
		HaikuModel: opts.HaikuModel, Endpoint: opts.Endpoint, Timeout: opts.Timeout,
		Prompt: prompt.Text,
	}

	if pf := preflight(ctx, req); pf.Err() != nil {
		var b strings.Builder
		fmt.Fprintf(&b, "preflight refused %s for %s; nothing was launched and nothing changed", opts.Adapter, id)
		for _, c := range pf.Checks {
			if c.Err != nil {
				fmt.Fprintf(&b, "\n  failed: %s: %s", c.Name, c.Err)
			}
		}
		return 1, errors.New(redact(b.String(), environ()))
	}

	start := now()
	logPath := workprompt.LogPath(root, id, opts.Adapter, start)
	logFile, err := workprompt.OpenLog(logPath, workprompt.Header{
		TaskID: id, Adapter: opts.Adapter, Model: opts.Model, Timeout: opts.Timeout, Start: start,
	})
	if err != nil {
		return 1, err
	}
	defer logFile.Close()
	// OpenLog may pick a suffixed name when the base name is taken, so print
	// and use the path it actually opened.
	logPath = logFile.Name()
	fmt.Fprintf(out, "log: %s\n", logPath)

	res := run(ctx, req)
	secrets := environ()
	output := redact(res.Output, secrets)
	note := redact(res.Note, secrets)
	code := "none"
	if res.ExitCode != nil {
		code = fmt.Sprint(*res.ExitCode)
	}
	if _, err := fmt.Fprintf(logFile, "State: %s\nExit code: %s\nNote: %s\n\n----- adapter output -----\n%s\n", res.State, code, note, output); err != nil {
		return 1, fmt.Errorf("write log %s: %w", logPath, err)
	}

	exit := 1
	switch {
	case res.State == common.StateExit && res.ExitCode != nil && *res.ExitCode == 0:
		exit = 0
		fmt.Fprintf(out, "worker exited 0 (a claim by the worker, not a pass)\n")
		reportHead(out, worktree, prompt.BaseCommit)
		fmt.Fprintf(out, "next: taskfactory verify %s\n", id)
	case res.State == common.StateExit:
		fmt.Fprintf(out, "worker failed: exit code %s\nlog: %s\n", code, logPath)
		reportHead(out, worktree, prompt.BaseCommit)
		fmt.Fprintf(out, "next: taskfactory verify %s\n", id)
	default:
		fmt.Fprintf(out, "worker %s: %s\nlog: %s\n", res.State, oneLine(note), logPath)
		reportHead(out, worktree, prompt.BaseCommit)
		fmt.Fprintf(out, "suggestion, not run: taskfactory fail %s --outcome BLOCKED --reason %q\n", id, oneLine(note))
	}
	return exit, nil
}

// oneLine collapses s to a single line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// reportHead prints the worker's commit when the worktree head moved past the
// Claim base commit. It only reads. A failed read is not a refusal.
func reportHead(out io.Writer, worktree, base string) {
	head, err := gitOut(worktree, "rev-parse", "HEAD")
	if err != nil || head == base {
		return
	}
	info, err := gitOut(worktree, "log", "-1", "--format=%h %s")
	if err != nil {
		return
	}
	fmt.Fprintf(out, "worker commit: %s\n", info)
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	data, err := cmd.Output()
	return strings.TrimSpace(string(data)), err
}

// credentialNames marks environment variable names whose values are secrets.
var credentialNames = []string{"TOKEN", "KEY", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL"}

// redact replaces the value of every credential-looking environment variable
// in s. Values shorter than four bytes are skipped to avoid mangling text.
func redact(s string, environ []string) string {
	for _, kv := range environ {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || len(value) < 4 {
			continue
		}
		upper := strings.ToUpper(name)
		for _, marker := range credentialNames {
			if strings.Contains(upper, marker) {
				s = strings.ReplaceAll(s, value, "[redacted]")
				break
			}
		}
	}
	return s
}

func realPreflight(ctx context.Context, r Request) common.PreflightResult {
	switch r.Adapter {
	case "omp":
		return omp.Preflight(ctx, ompRequest(r), omp.Options{Endpoint: r.Endpoint})
	case "pi":
		return pi.Preflight(ctx, piRequest(r), pi.Options{Endpoint: r.Endpoint})
	default:
		return claude.Preflight(ctx, claudeRequest(r), claude.Options{Endpoint: r.Endpoint})
	}
}

func realRun(ctx context.Context, r Request) common.Result {
	switch r.Adapter {
	case "omp":
		return omp.Run(ctx, ompRequest(r), omp.Options{Endpoint: r.Endpoint})
	case "pi":
		return pi.Run(ctx, piRequest(r), pi.Options{Endpoint: r.Endpoint})
	default:
		return claude.Run(ctx, claudeRequest(r), claude.Options{Endpoint: r.Endpoint})
	}
}

func ompRequest(r Request) omp.Request {
	return omp.Request{TaskID: r.TaskID, Worktree: r.Worktree, Model: r.Model, Timeout: r.Timeout, Prompt: r.Prompt}
}

func piRequest(r Request) pi.Request {
	return pi.Request{TaskID: r.TaskID, Worktree: r.Worktree, Model: r.Model, Timeout: r.Timeout, Prompt: r.Prompt}
}

func claudeRequest(r Request) claude.Request {
	return claude.Request{TaskID: r.TaskID, Worktree: r.Worktree, Model: r.Model, HaikuModel: r.HaikuModel, Timeout: r.Timeout, Prompt: r.Prompt}
}

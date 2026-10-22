package work

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
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
	// Stream receives adapter output as it arrives, already redacted. Nil means none.
	Stream io.Writer
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

	setTrace(opts.Verbose)
	defer setTrace(nil)
	tracef("read task %s and worker instructions under %s", id, root)
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

	if opts.DryRun {
		if opts.Would != nil {
			opts.Would("run the %s preflight", opts.Adapter)
			opts.Would("launch %s with model %s in %s", opts.Adapter, opts.Model, worktree)
			opts.Would("write log %s", workprompt.LogPath(root, id, opts.Adapter, now()))
		}
		return 0, nil
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

	// Output streams to the terminal and the log while the worker runs. The
	// header is already in the log, so hdrLen is where the body starts. The
	// State block goes in front of the body once the exit is known.
	hdrLen, err := logFile.Seek(0, io.SeekCurrent)
	if err != nil {
		return 1, fmt.Errorf("locate log body %s: %w", logPath, err)
	}
	secrets := environ()
	stream := &streamWriter{secrets: secrets, dst: []io.Writer{out, logFile}}
	req.Stream = stream
	res := run(ctx, req)
	stream.flush()
	if err := stream.err; err != nil {
		return 1, fmt.Errorf("write log %s: %w", logPath, err)
	}
	output := redact(res.Output, secrets)
	note := redact(res.Note, secrets)
	code := "none"
	if res.ExitCode != nil {
		code = fmt.Sprint(*res.ExitCode)
	}
	body := stream.logged.String()
	if body == "" {
		// The adapter returned output without streaming it, so log it whole.
		body = output
	}
	block := fmt.Sprintf("State: %s\nExit code: %s\nNote: %s\n\n----- adapter output -----\n%s\n", res.State, code, note, body)
	if err := rewriteBody(logFile, hdrLen, block); err != nil {
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

// streamWriter takes adapter output as it arrives, redacts it one complete line
// at a time, and writes each line to every destination. The bytes written to
// the log are kept in logged, so the final log can place the State block in
// front of them. A write error is kept in err and does not stop the worker.
type streamWriter struct {
	secrets []string
	dst     []io.Writer
	pending []byte
	logged  strings.Builder
	err     error
}

func (w *streamWriter) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		w.emit(w.pending[:i+1])
		w.pending = w.pending[i+1:]
	}
	return len(p), nil
}

// flush emits a final partial line, such as output that lacks a newline.
func (w *streamWriter) flush() {
	if len(w.pending) > 0 {
		w.emit(w.pending)
		w.pending = nil
	}
}

func (w *streamWriter) emit(line []byte) {
	text := redact(string(line), w.secrets)
	w.logged.WriteString(text)
	for _, d := range w.dst {
		if _, err := io.WriteString(d, text); err != nil && w.err == nil {
			w.err = err
		}
	}
}

// rewriteBody cuts the log back to hdrLen and writes block there, so the State
// block sits between the header and the adapter output.
func rewriteBody(f *os.File, hdrLen int64, block string) error {
	if err := f.Truncate(hdrLen); err != nil {
		return err
	}
	if _, err := f.Seek(hdrLen, io.SeekStart); err != nil {
		return err
	}
	_, err := io.WriteString(f, block)
	return err
}

// trace is the verbose sink of the running call; it is nil unless Options.Verbose is set.
var trace atomic.Pointer[func(format string, args ...any)]

// setTrace installs or clears the sink; it is safe for concurrent calls.
func setTrace(f func(format string, args ...any)) {
	if f == nil {
		trace.Store(nil)
		return
	}
	trace.Store(&f)
}

func tracef(format string, args ...any) {
	if f := trace.Load(); f != nil {
		(*f)(format, args...)
	}
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
	tracef("git -C %s %s", dir, strings.Join(args, " "))
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	data, err := cmd.Output()
	return strings.TrimSpace(string(data)), err
}

// Redact is redact for callers outside this package, such as --verbose output.
func Redact(s string, environ []string) string { return redact(s, environ) }

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
		return omp.Run(ctx, ompRequest(r), omp.Options{Endpoint: r.Endpoint, Stream: r.Stream})
	case "pi":
		return pi.Run(ctx, piRequest(r), pi.Options{Endpoint: r.Endpoint, Stream: r.Stream})
	default:
		return claude.Run(ctx, claudeRequest(r), claude.Options{Endpoint: r.Endpoint, Stream: r.Stream})
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

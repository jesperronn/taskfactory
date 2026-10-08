package claude

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	progressNote   = "progress: captured -p text output only; the adapter does not use --output-format stream-json"
	timeoutNote    = "wall-clock timeout: enforced by the adapter killing the child; the claude CLI has no time flag and --max-budget-usd (a cost cap) is not used, so the timeout is not a harness-level bound"
	permissionNote = "permissions: --permission-mode acceptEdits with an explicit --allowedTools list (assumption; docs/local-workers-v1.md says manual)"
)

// Run performs the preflight and, only if every check passes, launches one
// Claude Code run in req.Worktree with the prompt on stdin. A refused preflight
// returns StateBlocked with no launch. A run that exceeds req.Timeout returns
// StateStalled. Every result note records the haiku-model mapping used.
func Run(ctx context.Context, req Request, opts Options) Result {
	res := run(ctx, req, opts)
	mapping := fmt.Sprintf("haiku mapping: ANTHROPIC_DEFAULT_HAIKU_MODEL=%s, so haiku side calls use it and not only the requested model %s", req.HaikuModel, req.Model)
	res.Note = joinNotes(res.Note, timeoutNote, mapping, permissionNote)
	return res
}

func joinNotes(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "; ")
}

func run(ctx context.Context, req Request, opts Options) Result {
	argv, err := NewArgvBuilder().Argv(req)
	if err != nil {
		return Result{State: StateBlocked, Note: err.Error()}
	}
	if info, err := os.Stat(req.Worktree); err != nil || !info.IsDir() {
		return Result{State: StateBlocked, Note: fmt.Sprintf("worktree %s is not a directory", req.Worktree)}
	}
	if pf := Preflight(ctx, req, opts); pf.Err() != nil {
		return Result{State: StateBlocked, Note: "preflight refused launch: " + pf.Err().Error()}
	}
	baseURL, err := BaseURL(opts.endpoint())
	if err != nil {
		return Result{State: StateBlocked, Note: err.Error()}
	}
	binPath, err := exec.LookPath(Binary)
	if err != nil {
		return Result{State: StateBlocked, Note: err.Error()}
	}

	runCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, binPath, argv...)
	cmd.Dir = req.Worktree
	cmd.Env = Environ(os.Environ(), baseURL, opts.token(), req)
	cmd.Stdin = strings.NewReader(req.Prompt)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	runErr := cmd.Run()
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return Result{State: StateStalled, Output: out.String(), Note: "timeout exceeded; " + progressNote}
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code := exitErr.ExitCode()
			return Result{State: StateExit, ExitCode: &code, Output: out.String(), Note: progressNote}
		}
		return Result{State: StateBlocked, Output: out.String(), Note: "process did not start: " + runErr.Error()}
	}
	code := 0
	return Result{State: StateExit, ExitCode: &code, Output: out.String(), Note: progressNote}
}

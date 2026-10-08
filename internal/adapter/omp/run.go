package omp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Run performs the preflight and, only if every check passes, launches one
// OMP run in req.Worktree. A refused preflight returns StateBlocked with no
// launch. A run that exceeds req.Timeout returns StateStalled. Every result
// note states the side-call model mapping from the OMP config.
func Run(ctx context.Context, req Request, opts Options) Result {
	res := run(ctx, req, opts)
	home := opts.Home
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	side := sideCallNote(home)
	if res.Note == "" {
		res.Note = side
	} else {
		res.Note += "; " + side
	}
	return res
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

	runCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()
	binPath, err := exec.LookPath(Binary)
	if err != nil {
		return Result{State: StateBlocked, Note: err.Error()}
	}
	cmd := exec.CommandContext(runCtx, binPath, argv...)
	cmd.Dir = req.Worktree
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	const progressNote = "progress: captured -p output only; the adapter does not use --mode rpc"
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

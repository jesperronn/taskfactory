package pi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Run performs the preflight and, only if every check passes, launches one
// Pi run in req.Worktree. A refused preflight returns StateBlocked with no
// launch. A run that exceeds req.Timeout returns StateStalled; the bound is
// enforced by this adapter because Pi has no observed timeout flag.
func Run(ctx context.Context, req Request, opts Options) Result {
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

	const notes = "progress: captured -p output only; pi --mode rpc was not used or observed; " +
		"timeout is enforced by the adapter (wall-clock, no pi timeout flag observed); " +
		"no approval flag passed (manual)"
	runErr := cmd.Run()
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return Result{State: StateStalled, Output: out.String(), Note: "timeout exceeded; " + notes}
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code := exitErr.ExitCode()
			return Result{State: StateExit, ExitCode: &code, Output: out.String(), Note: notes}
		}
		return Result{State: StateBlocked, Output: out.String(), Note: "process did not start: " + runErr.Error()}
	}
	code := 0
	return Result{State: StateExit, ExitCode: &code, Output: out.String(), Note: notes}
}

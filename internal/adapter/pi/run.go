package pi

import (
	"context"

	"taskfactory/internal/adapter/common"
)

// piNotes records what the adapter could and could not observe about Pi.
const piNotes = "progress: captured -p output only; pi --mode rpc was not used or observed; " +
	"timeout is enforced by the adapter (wall-clock, no pi timeout flag observed); " +
	"no approval flag passed (manual)"

// Run performs the preflight and, only if every check passes, launches one
// Pi run in req.Worktree. A refused preflight returns StateBlocked with no
// launch. A run that exceeds req.Timeout returns StateStalled; the bound is
// enforced by this adapter because Pi has no observed timeout flag.
func Run(ctx context.Context, req Request, opts Options) Result {
	argv, err := NewArgvBuilder().Argv(req)
	if err != nil {
		return Result{State: StateBlocked, Note: err.Error()}
	}
	if err := common.CheckDir(req.Worktree); err != nil {
		return Result{State: StateBlocked, Note: err.Error()}
	}
	if pf := Preflight(ctx, req, opts); pf.Err() != nil {
		return Result{State: StateBlocked, Note: "preflight refused launch: " + pf.Err().Error()}
	}
	return common.Launch(ctx, common.LaunchSpec{
		Binary:  Binary,
		Argv:    argv,
		Dir:     req.Worktree,
		Timeout: req.Timeout,
		Notes:   piNotes,
	})
}

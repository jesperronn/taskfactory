package omp

import (
	"context"
	"os"

	"taskfactory/internal/adapter/common"
)

// progressNote records the only progress channel the adapter observes.
const progressNote = "progress: captured -p output only; the adapter does not use --mode rpc"

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
		Notes:   progressNote,
	})
}

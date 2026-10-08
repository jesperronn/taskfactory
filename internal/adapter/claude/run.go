package claude

import (
	"context"
	"fmt"
	"os"
	"strings"

	"taskfactory/internal/adapter/common"
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
	if err := common.CheckDir(req.Worktree); err != nil {
		return Result{State: StateBlocked, Note: err.Error()}
	}
	if pf := Preflight(ctx, req, opts); pf.Err() != nil {
		return Result{State: StateBlocked, Note: "preflight refused launch: " + pf.Err().Error()}
	}
	baseURL, err := BaseURL(opts.endpoint())
	if err != nil {
		return Result{State: StateBlocked, Note: err.Error()}
	}
	return common.Launch(ctx, common.LaunchSpec{
		Binary:  Binary,
		Argv:    argv,
		Dir:     req.Worktree,
		Env:     Environ(os.Environ(), baseURL, opts.token(), req),
		Stdin:   strings.NewReader(req.Prompt),
		Timeout: req.Timeout,
		Notes:   progressNote,
	})
}

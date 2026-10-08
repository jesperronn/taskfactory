// Package omp implements the OMP local worker adapter described in
// docs/local-workers-v1.md. It builds the explicit-model launch argv, runs a
// preflight that refuses to launch when the binary, model, or endpoint is
// unavailable, and maps the process outcome to the v1 result states.
//
// The argv builder interface, Request, Result, and PreflightResult are kept
// adapter-neutral so the Pi and Claude Code adapters can follow the same shape.
// The shared types and launch skeleton live in internal/adapter/common.
package omp

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"taskfactory/internal/adapter/common"
)

// State is the v1 result state of one adapter run.
type State = common.State

const (
	// StateExit means the harness ran and its --print mode exited with a code.
	StateExit = common.StateExit
	// StateBlocked means the preflight refused, so nothing was launched.
	StateBlocked = common.StateBlocked
	// StateStalled means the run exceeded its timeout.
	StateStalled = common.StateStalled
)

// DefaultEndpoint is the project-local oMLX server address.
const DefaultEndpoint = "127.0.0.1:8000"

// Request is one launch request. Every field is explicit; there is no
// adapter auto-detection and no default model.
type Request struct {
	TaskID   string
	Worktree string        // canonical absolute path of the claimed worktree
	Model    string        // explicit model selector, e.g. omlx/Ornith-1.5-35B-A3B-MLX-4bit
	Timeout  time.Duration // stall bound, at least one second
	Prompt   string        // task body and included files
}

// Result is the outcome of one run. Commit and changed-file capture belong to
// the TF-027 recorder and are not produced here.
type Result = common.Result

// ArgvBuilder builds the argv for one launch. Implementations must reject a
// request with no explicit model and must never add a fallback model.
type ArgvBuilder interface {
	// Argv returns the arguments for the harness binary, excluding argv[0].
	Argv(req Request) ([]string, error)
}

// PreflightResult lists every preflight check in the order it ran.
type PreflightResult = common.PreflightResult

// PreflightCheck is one named check and its error, nil when it passed.
type PreflightCheck = common.PreflightCheck

// argvBuilder is the OMP implementation of ArgvBuilder.
type argvBuilder struct{}

// NewArgvBuilder returns the OMP argv builder. It selects only the explicit
// model and never adds --smol, --slow, --plan, or --auto-approve.
func NewArgvBuilder() ArgvBuilder {
	return argvBuilder{}
}

// Argv builds:
// omp --model <model> --cwd <worktree> --max-time <seconds> --no-session -p <prompt>
func (argvBuilder) Argv(req Request) ([]string, error) {
	if req.Model == "" {
		return nil, errors.New("omp: model must be explicit; no default or fallback model")
	}
	if !filepath.IsAbs(req.Worktree) {
		return nil, fmt.Errorf("omp: worktree %q must be an absolute path", req.Worktree)
	}
	if req.Timeout < time.Second {
		return nil, fmt.Errorf("omp: timeout %s must be at least one second", req.Timeout)
	}
	if req.Prompt == "" {
		return nil, errors.New("omp: prompt must be non-empty")
	}
	seconds := strconv.FormatInt(int64(req.Timeout/time.Second), 10)
	return []string{
		"--model", req.Model,
		"--cwd", req.Worktree,
		"--max-time", seconds,
		"--no-session",
		"-p", req.Prompt,
	}, nil
}

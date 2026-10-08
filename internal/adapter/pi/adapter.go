// Package pi implements the Pi local worker adapter described in
// docs/local-workers-v1.md. It builds the explicit-model launch argv, runs a
// preflight that refuses to launch when the binary, model, or endpoint is
// unavailable, and maps the process outcome to the v1 result states.
//
// Pi exposes no timeout flag and no working-directory flag in `pi --help`, so
// the adapter enforces the wall-clock bound with a context and sets the
// process working directory itself.
package pi

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"taskfactory/internal/adapter/common"
)

// State is the v1 result state of one adapter run.
type State = common.State

const (
	// StateExit means Pi ran in -p mode and exited with a code.
	StateExit = common.StateExit
	// StateBlocked means the preflight refused, so nothing was launched.
	StateBlocked = common.StateBlocked
	// StateStalled means the run exceeded the adapter's wall-clock bound.
	StateStalled = common.StateStalled
)

// Binary is the harness executable name resolved on PATH.
const Binary = "pi"

// Provider is the Pi provider id of the project-local oMLX server, as already
// configured in Pi's models file.
const Provider = "omlx"

// DefaultEndpoint is the project-local oMLX server address.
const DefaultEndpoint = "127.0.0.1:8000"

// Thinking is the fixed thinking level used for every run.
const Thinking = "low"

// Request is one launch request. Every field is explicit; there is no adapter
// auto-detection and no default model.
type Request struct {
	TaskID   string
	Worktree string        // canonical absolute path of the claimed worktree
	Model    string        // bare model id, e.g. Ornith-1.5-35B-A3B-MLX-4bit
	Timeout  time.Duration // wall-clock bound enforced by the adapter, at least one second
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

// argvBuilder is the Pi implementation of ArgvBuilder.
type argvBuilder struct{}

// NewArgvBuilder returns the Pi argv builder. It selects only the explicit
// model on the omlx provider and never adds --models, --tools, --approve, or
// any fallback model.
func NewArgvBuilder() ArgvBuilder {
	return argvBuilder{}
}

// Argv builds:
// pi --provider omlx --model <model> --thinking low --no-session -p <prompt>
// The worktree is not an argument; Run sets it as the process working
// directory because Pi has no observed cwd flag.
func (argvBuilder) Argv(req Request) ([]string, error) {
	if req.Model == "" {
		return nil, errors.New("pi: model must be explicit; no default or fallback model")
	}
	if strings.ContainsAny(req.Model, "/:") {
		return nil, fmt.Errorf("pi: model %q must be a bare id; the provider is set to %s", req.Model, Provider)
	}
	if !filepath.IsAbs(req.Worktree) {
		return nil, fmt.Errorf("pi: worktree %q must be an absolute path", req.Worktree)
	}
	if req.Timeout < time.Second {
		return nil, fmt.Errorf("pi: timeout %s must be at least one second", req.Timeout)
	}
	if req.Prompt == "" {
		return nil, errors.New("pi: prompt must be non-empty")
	}
	return []string{
		"--provider", Provider,
		"--model", req.Model,
		"--thinking", Thinking,
		"--no-session",
		"-p", req.Prompt,
	}, nil
}

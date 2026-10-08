// Package claude implements the Claude Code local worker adapter described in
// docs/local-workers-v1.md and TF-026. It routes one Claude Code run to a
// project-local oMLX server on a loopback address with an explicit model and
// no fallback, runs a preflight that refuses to launch when the binary, the
// endpoint, or a requested model is unavailable, and maps the process outcome
// to the v1 result states.
//
// DECISION ASSUMPTION (the owner has not answered yet): the adapter passes
// `--permission-mode acceptEdits` together with an explicit --allowedTools
// list, because that is the value the TF-015 trials ran. docs/local-workers-v1.md
// says `manual`. Revisit this once the owner reconciles the two.
//
// The prompt always goes on stdin. --allowedTools and --disallowedTools are
// variadic and would swallow a positional prompt. The child environment is
// built per run and handed only to the child process; nothing is written to
// files or global state.
//
// The model catalog check queries the endpoint's /v1/models. `claude --help`
// and `omlx launch --help` do not list registered models, so help output
// cannot decide the outcome; the endpoint listing does.
package claude

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"
)

// State is the v1 result state of one adapter run.
type State string

const (
	// StateExit means the harness ran and its --print mode exited with a code.
	StateExit State = "exit"
	// StateBlocked means the preflight refused, so nothing was launched.
	StateBlocked State = "blocked"
	// StateStalled means the run exceeded its timeout.
	StateStalled State = "stalled"
)

// Binary is the harness executable name resolved on PATH.
const Binary = "claude"

// DefaultEndpoint is the project-local oMLX server address.
const DefaultEndpoint = "127.0.0.1:8000"

// PermissionMode is the --permission-mode value passed to Claude Code. See the
// package comment: this is an unconfirmed assumption that mirrors the trials.
const PermissionMode = "acceptEdits"

// DisallowedTools is the --disallowedTools list. LSP is denied as observed in
// the trials.
var DisallowedTools = []string{"LSP"}

// AllowedTools is the smallest tool list that lets a task edit files, run the
// repository wrappers bin/test and bin/lint, and commit its result. The trials
// did not record the list used, so this is a choice made by the adapter.
var AllowedTools = []string{
	"Read",
	"Edit",
	"Write",
	"Glob",
	"Grep",
	"Bash(bin/test)",
	"Bash(bin/lint)",
	"Bash(git status)",
	"Bash(git diff)",
	"Bash(git add)",
	"Bash(git commit)",
}

// Request is one launch request. Every field is explicit; there is no adapter
// auto-detection and no default or fallback model.
type Request struct {
	TaskID     string
	Worktree   string        // canonical absolute path of the claimed worktree
	Model      string        // explicit model id for Opus and Sonnet tiers and --model
	HaikuModel string        // explicit model id for the haiku tier and side calls
	Timeout    time.Duration // stall bound, at least one second
	Prompt     string        // task body and included files, sent on stdin
}

// Result is the outcome of one run. Commit and changed-file capture belong to
// the TF-027 recorder and are not produced here.
type Result struct {
	State    State
	ExitCode *int   // shell exit code, or nil if the process never started
	Output   string // captured -p output, the only progress channel for this adapter
	Note     string // human context, including refusal reasons and model mapping
}

// ArgvBuilder builds the argv for one launch. Implementations must reject a
// request with no explicit model and must never add a fallback model.
type ArgvBuilder interface {
	// Argv returns the arguments for the harness binary, excluding argv[0].
	// The prompt is not part of the argv; it is written to stdin.
	Argv(req Request) ([]string, error)
}

// PreflightResult lists every preflight check in the order it ran.
type PreflightResult struct {
	Checks []PreflightCheck
}

// PreflightCheck is one named check and its error, nil when it passed.
type PreflightCheck struct {
	Name string
	Err  error
}

// Err returns all failed checks joined into one error, or nil when every check
// passed. A non-nil result means the run must be refused.
func (p PreflightResult) Err() error {
	var errs []error
	for _, c := range p.Checks {
		if c.Err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", c.Name, c.Err))
		}
	}
	return errors.Join(errs...)
}

// argvBuilder is the Claude Code implementation of ArgvBuilder.
type argvBuilder struct{}

// NewArgvBuilder returns the Claude Code argv builder. It selects only the
// explicit model and never adds --fallback-model.
func NewArgvBuilder() ArgvBuilder {
	return argvBuilder{}
}

// Argv builds:
// claude -p --model <model> --disallowedTools LSP --permission-mode acceptEdits --allowedTools <list>
// The prompt goes on stdin; see Stdin.
func (argvBuilder) Argv(req Request) ([]string, error) {
	if err := validate(req); err != nil {
		return nil, err
	}
	return []string{
		"-p",
		"--model", req.Model,
		"--disallowedTools", strings.Join(DisallowedTools, ","),
		"--permission-mode", PermissionMode,
		"--allowedTools", strings.Join(AllowedTools, ","),
	}, nil
}

// validate rejects a request that could not launch a single explicit run.
func validate(req Request) error {
	if req.Model == "" {
		return errors.New("claude: model must be explicit; no default or fallback model")
	}
	if req.HaikuModel == "" {
		return errors.New("claude: haiku model must be explicit; the haiku alias must not pick a model implicitly")
	}
	if !filepath.IsAbs(req.Worktree) {
		return fmt.Errorf("claude: worktree %q must be an absolute path", req.Worktree)
	}
	if req.Timeout < time.Second {
		return fmt.Errorf("claude: timeout %s must be at least one second", req.Timeout)
	}
	if req.Prompt == "" {
		return errors.New("claude: prompt must be non-empty")
	}
	return nil
}

// BaseURL returns the http URL of endpoint (host:port) when its host is an IP
// literal in a loopback range. Any other host, including a hostname such as
// localhost, is refused so that no request can leave the machine.
func BaseURL(endpoint string) (string, error) {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", fmt.Errorf("endpoint %q must be host:port: %w", endpoint, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", fmt.Errorf("base URL host %q is not a loopback IP address", host)
	}
	return "http://" + net.JoinHostPort(host, port), nil
}

// Environ returns the environment for the child process only. Every ANTHROPIC_*
// variable and CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC is removed from parent
// first, so a leaked key or hosted endpoint cannot redirect the run. The local
// routing set is then added: base URL, placeholder token, the explicit model for
// the Opus and Sonnet tiers, the explicit haiku model, and the traffic switch.
func Environ(parent []string, baseURL, token string, req Request) []string {
	env := make([]string, 0, len(parent)+6)
	for _, kv := range parent {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "ANTHROPIC_") || name == "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC" {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"ANTHROPIC_BASE_URL="+baseURL,
		"ANTHROPIC_AUTH_TOKEN="+token,
		"ANTHROPIC_DEFAULT_OPUS_MODEL="+req.Model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL="+req.Model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL="+req.HaikuModel,
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
	)
}

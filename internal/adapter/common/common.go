// Package common holds the launch plumbing shared by the OMP, Pi and Claude Code
// local worker adapters described in docs/local-workers-v1.md: the v1 result
// states and result types, the preflight result shape, a TCP endpoint check,
// option defaulting, and the run skeleton that starts a harness process in a
// worktree and maps its outcome to exit, blocked, or stalled.
//
// The package knows nothing about a particular harness. Each adapter keeps its
// own argv, model listing, preflight wording, and progress notes.
package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"
)

// State is the v1 result state of one adapter run.
type State string

const (
	// StateExit means the harness ran and exited with a code.
	StateExit State = "exit"
	// StateBlocked means the preflight refused, so nothing was launched.
	StateBlocked State = "blocked"
	// StateStalled means the run exceeded its wall-clock bound.
	StateStalled State = "stalled"
)

// PreflightTimeout bounds each preflight check that talks to a process or the
// network.
const PreflightTimeout = 30 * time.Second

// KillGrace bounds how long Launch waits for output pipes to close after the
// bound expires, so a child process that outlives the harness cannot hang it.
const KillGrace = 2 * time.Second

// Result is the outcome of one run. Commit and changed-file capture belong to
// the TF-027 recorder and are not produced here.
type Result struct {
	State    State
	ExitCode *int   // shell exit code, or nil if the process never started
	Output   string // captured output, the only progress channel for the adapters
	Note     string // human context, including refusal reasons
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

// Endpoint returns configured, or def when configured is empty. It is the
// option defaulting every adapter applies to its endpoint.
func Endpoint(configured, def string) string {
	if configured == "" {
		return def
	}
	return configured
}

// CheckDir returns an error unless path names an existing directory.
func CheckDir(path string) error {
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return fmt.Errorf("worktree %s is not a directory", path)
	}
	return nil
}

// BinaryCheck resolves binary on PATH. It returns the resolved path, or an
// empty string when the binary is missing, together with the named check.
func BinaryCheck(binary string) (string, PreflightCheck) {
	path, err := exec.LookPath(binary)
	return path, PreflightCheck{Name: "adapter binary " + binary, Err: err}
}

// EndpointCheck reports whether a TCP connection to endpoint opens. A nil dial
// means DialTCP. The check is bounded by PreflightTimeout.
func EndpointCheck(ctx context.Context, endpoint string, dial func(context.Context, string) error) PreflightCheck {
	if dial == nil {
		dial = DialTCP
	}
	dialCtx, cancel := context.WithTimeout(ctx, PreflightTimeout)
	defer cancel()
	return PreflightCheck{Name: "endpoint " + endpoint, Err: dial(dialCtx, endpoint)}
}

// DialTCP succeeds when a TCP connection to addr opens and closes cleanly.
func DialTCP(ctx context.Context, addr string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("endpoint %s is unreachable: %w", addr, err)
	}
	return conn.Close()
}

// LaunchSpec describes one harness process. The zero Env inherits the parent
// environment, and a nil Stdin gives the process no standard input.
type LaunchSpec struct {
	Binary  string        // executable name, resolved on PATH
	Argv    []string      // arguments, excluding argv[0]
	Dir     string        // process working directory
	Env     []string      // complete child environment; nil inherits
	Stdin   io.Reader     // standard input; nil gives none
	Timeout time.Duration // wall-clock bound enforced by Launch
	Notes   string        // progress and mapping notes, kept on exit and stall
}

// Launch starts the harness once and maps its outcome. A binary that does not
// resolve on PATH, or a process that cannot start, returns StateBlocked. A run
// that exceeds Timeout is killed and returns StateStalled, with the wait for
// output pipes bounded by KillGrace. Any other process exit returns StateExit
// with its code. Launch does not check preflight; callers run it first.
func Launch(ctx context.Context, spec LaunchSpec) Result {
	binPath, err := exec.LookPath(spec.Binary)
	if err != nil {
		return Result{State: StateBlocked, Note: err.Error()}
	}

	runCtx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, binPath, spec.Argv...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	cmd.Stdin = spec.Stdin
	cmd.WaitDelay = KillGrace
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	runErr := cmd.Run()
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return Result{State: StateStalled, Output: out.String(), Note: "timeout exceeded; " + spec.Notes}
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code := exitErr.ExitCode()
			return Result{State: StateExit, ExitCode: &code, Output: out.String(), Note: spec.Notes}
		}
		return Result{State: StateBlocked, Output: out.String(), Note: "process did not start: " + runErr.Error()}
	}
	code := 0
	return Result{State: StateExit, ExitCode: &code, Output: out.String(), Note: spec.Notes}
}

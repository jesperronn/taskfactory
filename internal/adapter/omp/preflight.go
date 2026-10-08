package omp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"time"
)

// Binary is the harness executable name resolved on PATH.
const Binary = "omp"

// Options configures the preflight and run. The zero value uses the real
// binary on PATH and the default loopback endpoint.
type Options struct {
	// Endpoint is the host:port of the oMLX server. Empty means DefaultEndpoint.
	Endpoint string
	// Dial reports whether addr accepts a TCP connection. Nil means a real
	// net.Dialer. Tests inject it so no socket is opened.
	Dial func(ctx context.Context, addr string) error
	// Home is the directory holding .omp/agent/config.yml. Empty means the
	// user's home directory. Tests set it to a temporary directory.
	Home string
}

const preflightTimeout = 30 * time.Second

// Preflight runs every check before launch and never starts a model run. The
// checks are: the binary resolves on PATH, the model is listed by
// `omp models --json`, and the project-local endpoint accepts connections.
func Preflight(ctx context.Context, req Request, opts Options) PreflightResult {
	var result PreflightResult
	binPath, err := exec.LookPath(Binary)
	result.Checks = append(result.Checks, PreflightCheck{
		Name: "adapter binary " + Binary,
		Err:  err,
	})

	var modelErr error
	if err == nil {
		modelErr = checkModelListed(ctx, binPath, req.Model)
	} else {
		modelErr = errors.New("skipped because adapter binary is unavailable")
	}
	result.Checks = append(result.Checks, PreflightCheck{
		Name: "model " + req.Model,
		Err:  modelErr,
	})

	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	dial := opts.Dial
	if dial == nil {
		dial = dialTCP
	}
	dialCtx, cancel := context.WithTimeout(ctx, preflightTimeout)
	defer cancel()
	result.Checks = append(result.Checks, PreflightCheck{
		Name: "endpoint " + endpoint,
		Err:  dial(dialCtx, endpoint),
	})
	return result
}

// checkModelListed returns nil only when the selector is in the OMP catalog.
func checkModelListed(ctx context.Context, binPath, model string) error {
	listCtx, cancel := context.WithTimeout(ctx, preflightTimeout)
	defer cancel()
	cmd := exec.CommandContext(listCtx, binPath, "models", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("list models with %s models --json: %w: %s", binPath, err, bytes.TrimSpace(stderr.Bytes()))
	}
	var catalog struct {
		Models []struct {
			Selector string `json:"selector"`
		} `json:"models"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &catalog); err != nil {
		return fmt.Errorf("parse %s models --json: %w", binPath, err)
	}
	for _, m := range catalog.Models {
		if m.Selector == model {
			return nil
		}
	}
	return fmt.Errorf("model %q is not listed by %s models", model, binPath)
}

// dialTCP succeeds when a TCP connection to addr opens and closes cleanly.
func dialTCP(ctx context.Context, addr string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("endpoint %s is unreachable: %w", addr, err)
	}
	return conn.Close()
}

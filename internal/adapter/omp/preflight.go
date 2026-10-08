package omp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"taskfactory/internal/adapter/common"
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

// Preflight runs every check before launch and never starts a model run. The
// checks are: the binary resolves on PATH, the model is listed by
// `omp models --json`, and the project-local endpoint accepts connections.
func Preflight(ctx context.Context, req Request, opts Options) PreflightResult {
	var result PreflightResult
	binPath, binCheck := common.BinaryCheck(Binary)
	result.Checks = append(result.Checks, binCheck)

	var modelErr error
	if binCheck.Err == nil {
		modelErr = checkModelListed(ctx, binPath, req.Model)
	} else {
		modelErr = errors.New("skipped because adapter binary is unavailable")
	}
	result.Checks = append(result.Checks, common.PreflightCheck{
		Name: "model " + req.Model,
		Err:  modelErr,
	})

	endpoint := common.Endpoint(opts.Endpoint, DefaultEndpoint)
	result.Checks = append(result.Checks, common.EndpointCheck(ctx, endpoint, opts.Dial))
	return result
}

// checkModelListed returns nil only when the selector is in the OMP catalog.
func checkModelListed(ctx context.Context, binPath, model string) error {
	listCtx, cancel := context.WithTimeout(ctx, common.PreflightTimeout)
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

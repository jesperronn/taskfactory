package pi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"taskfactory/internal/adapter/common"
)

// Options configures the preflight and run. The zero value uses the real
// binary on PATH and the default loopback endpoint.
type Options struct {
	// Endpoint is the host:port of the oMLX server. Empty means DefaultEndpoint.
	Endpoint string
	// Dial reports whether addr accepts a TCP connection. Nil means a real
	// net.Dialer. Tests inject it so no socket is opened.
	Dial func(ctx context.Context, addr string) error
}

// Preflight runs every check before launch and never starts a model run. The
// checks are: the binary resolves on PATH, the model is listed by
// `pi --list-models` under the omlx provider, and the project-local endpoint
// accepts connections.
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
		Name: "model " + Provider + "/" + req.Model,
		Err:  modelErr,
	})

	endpoint := common.Endpoint(opts.Endpoint, DefaultEndpoint)
	result.Checks = append(result.Checks, common.EndpointCheck(ctx, endpoint, opts.Dial))
	return result
}

// checkModelListed returns nil only when `pi --list-models` output has a line
// that names the bare model id together with the omlx provider. The listing
// format is not documented in `pi --help`; this matches either the
// provider/id form or a line containing both the provider and the id.
func checkModelListed(ctx context.Context, binPath, model string) error {
	listCtx, cancel := context.WithTimeout(ctx, common.PreflightTimeout)
	defer cancel()
	cmd := exec.CommandContext(listCtx, binPath, "--list-models")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("list models with %s --list-models: %w: %s", binPath, err, bytes.TrimSpace(stderr.Bytes()))
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if listsModel(line, model) {
			return nil
		}
	}
	return fmt.Errorf("model %q under provider %s is not listed by %s --list-models", model, Provider, binPath)
}

// listsModel reports whether one listing line names the model on the provider.
func listsModel(line, model string) bool {
	fields := strings.Fields(line)
	want := Provider + "/" + model
	hasProvider, hasModel := false, false
	for _, f := range fields {
		if f == want {
			return true
		}
		if f == Provider {
			hasProvider = true
		}
		if f == model {
			hasModel = true
		}
	}
	return hasProvider && hasModel
}

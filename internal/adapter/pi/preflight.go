package pi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
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

const preflightTimeout = 30 * time.Second

// Preflight runs every check before launch and never starts a model run. The
// checks are: the binary resolves on PATH, the model is listed by
// `pi --list-models` under the omlx provider, and the project-local endpoint
// accepts connections.
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
		Name: "model " + Provider + "/" + req.Model,
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

// checkModelListed returns nil only when `pi --list-models` output has a line
// that names the bare model id together with the omlx provider. The listing
// format is not documented in `pi --help`; this matches either the
// provider/id form or a line containing both the provider and the id.
func checkModelListed(ctx context.Context, binPath, model string) error {
	listCtx, cancel := context.WithTimeout(ctx, preflightTimeout)
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

// dialTCP succeeds when a TCP connection to addr opens and closes cleanly.
func dialTCP(ctx context.Context, addr string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("endpoint %s is unreachable: %w", addr, err)
	}
	return conn.Close()
}

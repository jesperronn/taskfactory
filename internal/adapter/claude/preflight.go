package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"

	"taskfactory/internal/adapter/common"
)

// Options configures the preflight and run. The zero value uses the real
// binary on PATH, the default loopback endpoint, and the operator's
// ANTHROPIC_AUTH_TOKEN. Tests inject the network functions so no socket opens.
type Options struct {
	// Endpoint is the host:port of the oMLX server. Empty means DefaultEndpoint.
	Endpoint string
	// AuthToken is the local placeholder token. Empty means the value of
	// ANTHROPIC_AUTH_TOKEN in the operator's environment. It is never written to
	// a task, evidence, or result file.
	AuthToken string
	// Dial reports whether addr accepts a TCP connection. Nil means a real
	// net.Dialer. It is only called for a loopback endpoint.
	Dial func(ctx context.Context, addr string) error
	// ListModels returns the model ids served at baseURL (GET /v1/models). Nil
	// means an HTTP GET. It is only called for a loopback endpoint.
	ListModels func(ctx context.Context, baseURL, token string) ([]string, error)
}

func (o Options) endpoint() string {
	return common.Endpoint(o.Endpoint, DefaultEndpoint)
}

func (o Options) token() string {
	if o.AuthToken != "" {
		return o.AuthToken
	}
	return os.Getenv("ANTHROPIC_AUTH_TOKEN")
}

// Preflight runs every check before launch and never starts a model run. The
// checks, in order, are: the claude binary resolves on PATH, the endpoint is a
// loopback IP, an auth token is available, the endpoint accepts connections,
// the explicit model is listed by /v1/models, and the explicit haiku model is
// listed too. Nothing is dialed or queried for a non-loopback endpoint.
func Preflight(ctx context.Context, req Request, opts Options) PreflightResult {
	var result PreflightResult
	_, binCheck := common.BinaryCheck(Binary)
	result.Checks = append(result.Checks, binCheck)

	endpoint := opts.endpoint()
	baseURL, baseErr := BaseURL(endpoint)
	result.Checks = append(result.Checks, common.PreflightCheck{Name: "loopback endpoint " + endpoint, Err: baseErr})

	var tokenErr error
	if opts.token() == "" {
		tokenErr = errors.New("ANTHROPIC_AUTH_TOKEN is not set; the local placeholder must come from the operator environment")
	}
	result.Checks = append(result.Checks, common.PreflightCheck{Name: "auth token", Err: tokenErr})

	skipped := func(reason string) error { return errors.New("skipped because " + reason) }
	if baseErr != nil {
		for _, name := range []string{"endpoint " + endpoint, "model " + req.Model, "haiku model " + req.HaikuModel} {
			result.Checks = append(result.Checks, common.PreflightCheck{Name: name, Err: skipped("the endpoint is not loopback")})
		}
		return result
	}

	endpointCheck := common.EndpointCheck(ctx, endpoint, opts.Dial)
	result.Checks = append(result.Checks, endpointCheck)

	listModels := opts.ListModels
	if listModels == nil {
		listModels = listModelsHTTP
	}
	var listed []string
	var listErr error
	if endpointCheck.Err == nil {
		listCtx, listCancel := context.WithTimeout(ctx, common.PreflightTimeout)
		defer listCancel()
		listed, listErr = listModels(listCtx, baseURL, opts.token())
	} else {
		listErr = skipped("the endpoint is unreachable")
	}
	result.Checks = append(result.Checks,
		common.PreflightCheck{Name: "model " + req.Model, Err: checkListed(req.Model, "model", baseURL, listed, listErr)},
		common.PreflightCheck{Name: "haiku model " + req.HaikuModel, Err: checkListed(req.HaikuModel, "haiku model", baseURL, listed, listErr)},
	)
	return result
}

// checkListed returns nil only when id is in the endpoint's model list.
func checkListed(id, role, baseURL string, listed []string, listErr error) error {
	if id == "" {
		return fmt.Errorf("%s is empty; it must be explicit", role)
	}
	if listErr != nil {
		return listErr
	}
	if !slices.Contains(listed, id) {
		return fmt.Errorf("%s %q is not registered at %s/v1/models", role, id, baseURL)
	}
	return nil
}

// listModelsHTTP fetches the model ids from GET <baseURL>/v1/models. The token
// is sent as a bearer header and never included in an error message.
func listModelsHTTP(ctx context.Context, baseURL, token string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: common.PreflightTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query %s/v1/models: %w", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("query %s/v1/models: HTTP %d", baseURL, resp.StatusCode)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("parse %s/v1/models: %w", baseURL, err)
	}
	ids := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

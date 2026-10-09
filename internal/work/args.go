// Package work launches a local worker adapter on a claimed task and reports
// what happened. It writes only the run log under .taskfactory/logs. It never
// stages, commits, verifies, integrates or fails a task; the commands it
// prints are suggestions for the operator.
package work

import (
	"fmt"
	"strings"
	"time"

	"taskfactory/internal/workprompt"
)

// DefaultTimeout is the worker bound when --timeout is not given.
const DefaultTimeout = 10 * time.Minute

// DefaultEndpoint is the project-local oMLX server address, shared by all
// three adapters.
const DefaultEndpoint = "127.0.0.1:8000"

// UsageError marks invalid arguments, which the CLI reports as exit code 2.
type UsageError struct{ msg string }

func (e UsageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return UsageError{fmt.Sprintf(format, args...)}
}

// Options holds the parsed flags of the work command.
type Options struct {
	Adapter    string
	Model      string
	HaikuModel string // required for the claude adapter only
	Endpoint   string // host:port, DefaultEndpoint when not given
	Timeout    time.Duration
}

// ParseArgs parses the arguments after "work": one task ID, a required
// --adapter and --model, and optional --timeout, --endpoint and --haiku-model.
// Both "--flag value" and "--flag=value" are accepted. Every flag may be given
// once only. Unknown or extra arguments are usage errors. There is no default
// adapter and no default model.
func ParseArgs(args []string) (string, Options, error) {
	var id string
	opts := Options{Endpoint: DefaultEndpoint, Timeout: DefaultTimeout}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")
		switch name {
		case "--adapter", "--model", "--haiku-model", "--endpoint", "--timeout":
		default:
			if strings.HasPrefix(arg, "-") {
				return "", Options{}, usagef("unknown argument %q", arg)
			}
			if id != "" {
				return "", Options{}, usagef("unexpected extra argument %q", arg)
			}
			id = arg
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return "", Options{}, usagef("%s requires a value", name)
			}
			i++
			value = args[i]
		}
		if seen[name] {
			if name == "--adapter" {
				return "", Options{}, usagef("exactly one --adapter is accepted; run one worker at a time")
			}
			return "", Options{}, usagef("%s may be given only once", name)
		}
		seen[name] = true
		switch name {
		case "--adapter":
			opts.Adapter = value
		case "--model":
			opts.Model = value
		case "--haiku-model":
			opts.HaikuModel = value
		case "--endpoint":
			opts.Endpoint = value
		case "--timeout":
			d, err := time.ParseDuration(value)
			if err != nil {
				return "", Options{}, usagef("--timeout %q is not a duration such as 10m", value)
			}
			if d < time.Second {
				return "", Options{}, usagef("--timeout must be at least 1s")
			}
			opts.Timeout = d
		}
	}
	if id == "" {
		return "", Options{}, usagef("missing task ID")
	}
	if opts.Adapter == "" {
		return "", Options{}, usagef("missing --adapter (one of %s)", strings.Join(workprompt.Adapters, ", "))
	}
	known := false
	for _, a := range workprompt.Adapters {
		known = known || a == opts.Adapter
	}
	if !known {
		return "", Options{}, usagef("unknown adapter %q (one of %s)", opts.Adapter, strings.Join(workprompt.Adapters, ", "))
	}
	if opts.Model == "" {
		return "", Options{}, usagef("missing --model")
	}
	if strings.ContainsAny(opts.Model, " \t\r\n") {
		return "", Options{}, usagef("--model must not contain whitespace")
	}
	if opts.Endpoint == "" || strings.ContainsAny(opts.Endpoint, " \t\r\n") {
		return "", Options{}, usagef("--endpoint must be a host:port without whitespace")
	}
	switch {
	case opts.Adapter == "claude" && opts.HaikuModel == "":
		return "", Options{}, usagef("--adapter claude requires --haiku-model")
	case opts.Adapter != "claude" && seen["--haiku-model"]:
		return "", Options{}, usagef("--haiku-model applies only to --adapter claude")
	case strings.ContainsAny(opts.HaikuModel, " \t\r\n"):
		return "", Options{}, usagef("--haiku-model must not contain whitespace")
	}
	return id, opts, nil
}

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"taskfactory/internal/work"
)

// commonFlags are the two flags every subcommand accepts. They are removed from
// the argument list here, so each subcommand's own parser never sees them and
// its usage errors stay exactly as they were.
type commonFlags struct {
	Verbose bool
	DryRun  bool
	// StateDir is the value of --state-dir, empty when the flag is absent.
	StateDir string
}

// stateDirFlag is the --state-dir value of the running command. main sets it
// once, before any command resolves the project root.
var stateDirFlag string

// resolveStateDir returns the state directory in discovery order: the
// --state-dir flag value, then TF_STATE_DIR, then "" for the default layout
// (state inside the project). The second result names the source.
func resolveStateDir(flagValue string, getenv func(string) string) (dir, source string) {
	if flagValue != "" {
		return flagValue, "--state-dir"
	}
	if v := getenv("TF_STATE_DIR"); v != "" {
		return v, "TF_STATE_DIR"
	}
	return "", "default"
}

// extractCommonFlags removes --verbose and --dry-run from args, wherever they
// appear, and returns the remaining arguments in their original order. A flag
// given twice counts once. A value form such as --dry-run=true is a usage error.
func extractCommonFlags(args []string) ([]string, commonFlags, error) {
	var flags commonFlags
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")
		switch name {
		case "--state-dir":
			if !hasValue {
				if i+1 >= len(args) {
					return nil, commonFlags{}, fmt.Errorf("--state-dir needs a directory")
				}
				i++
				value = args[i]
			}
			if value == "" {
				return nil, commonFlags{}, fmt.Errorf("--state-dir needs a directory")
			}
			flags.StateDir = value
		case "--verbose", "--dry-run":
			if hasValue {
				return nil, commonFlags{}, fmt.Errorf("%s takes no value, got %q", name, arg)
			}
			if name == "--verbose" {
				flags.Verbose = true
			} else {
				flags.DryRun = true
			}
		default:
			rest = append(rest, arg)
		}
	}
	return rest, flags, nil
}

// verboseOn is set once by main when --verbose is given.
var verboseOn bool

// verbosef writes one "verbose: " line to stderr. Credential values from the
// environment are redacted the way work redacts its output, and the line never
// spans more than one physical line.
func verbosef(format string, args ...any) {
	if !verboseOn {
		return
	}
	line := work.Redact(fmt.Sprintf(format, args...), os.Environ())
	line = strings.NewReplacer("\r", " ", "\n", " ").Replace(line)
	fmt.Fprintln(os.Stderr, "verbose: "+line)
}

// dryRunClosing is the last line of every successful dry run.
const dryRunClosing = "dry run: nothing changed"

// dryRun prints the "would: " lines and the closing line of a dry run, so the
// wording is the same for every command.
type dryRun struct{ out io.Writer }

// would prints one planned action.
func (d dryRun) would(format string, args ...any) {
	fmt.Fprintf(d.out, "would: %s\n", fmt.Sprintf(format, args...))
}

// done prints the closing line.
func (d dryRun) done() {
	fmt.Fprintln(d.out, dryRunClosing)
}

// hintsOff suppresses the next-step hints; a dry run never prints them.
var hintsOff bool

// verboseFunc returns verbosef when --verbose is on and nil otherwise, which is
// what the Options.Verbose fields of the internal packages expect.
func verboseFunc() func(string, ...any) {
	if !verboseOn {
		return nil
	}
	return verbosef
}

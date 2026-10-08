// Command taskfactory is the TaskFactory CLI. It coordinates software work into
// explicit, verifiable tasks that coding agents can execute safely in parallel.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// version is the stable development version string printed by --version. It is a
// fixed, human-readable constant; do not derive it from build metadata here so
// that output stays predictable across checkouts and builds.
const version = "dev"

// usageText is the help text printed by --help and -h. It names the command and
// both global flags so that "taskfactory --help" documents the supported
// top-level flags.
const usageText = `taskfactory coordinates software work into explicit, verifiable tasks.

Usage:
  taskfactory [global flags]
  taskfactory <command> [flags]

Global flags:
  -help, --help      print this usage and exit successfully.
  -version, --version print the CLI version string and exit successfully.`

// exitUsage is the exit code returned when global flags are used incorrectly or
// an unknown flag is supplied.
const exitUsage = 2

func main() {
	fs := flag.NewFlagSet("taskfactory", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	// Keep flag.Parse from printing the full usage text on invalid input.
	fs.Usage = func() {}

	help := fs.Bool("help", false, "print this usage and exit successfully")
	showVersion := fs.Bool("version", false, "print the CLI version string and exit successfully")

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "taskfactory: %v\n", err)
		os.Exit(exitUsage)
	}

	if *help {
		fmt.Fprint(os.Stdout, usageText)
		return
	}

	if *showVersion {
		fmt.Fprintln(os.Stdout, "taskfactory version "+version)
		return
	}

	fmt.Fprintln(os.Stderr, usageText)
	os.Exit(exitUsage)
}

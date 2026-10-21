# TF-001: Establish the Go CLI foundation

## Goal

Create the smallest buildable Go command with `--help` and `--version`, ready
for later TaskFactory subcommands.

## Dependencies

None

## Scope

In this repository, add `go.mod`, `cmd/taskfactory/main.go`, and focused Go
tests. Declare a supported Go version in `go.mod`. Use the standard library; add
no third-party modules. Make `taskfactory --help` print usage and exit
successfully, and `taskfactory --version` print a stable development version and
exit successfully. Invalid global flags must exit nonzero with a concise error.
Do not implement task lifecycle commands in this task.

## Constraints

Follow `docs/technical-spec-v1.md`. Keep the command usable from outside the
source checkout. Preserve the docs and task files.

## Success criteria

- `go run ./cmd/taskfactory --help` exits 0 and shows the command name and both
  global flags.
- `go run ./cmd/taskfactory --version` exits 0 and prints a version string.
- An unknown flag exits nonzero and gives a useful error.
- `go test ./...` and `go vet ./...` pass with no third-party modules.

## Verification

Run the four commands above, including one unknown flag, and record their exit
codes. Report changed files and the resulting commit.

# TF-042: Colorize CLI output with the standard library

## Goal

Make `taskfactory` usage and status output easier to scan, in the spirit of uv,
without adding a dependency.

## Dependencies

- TF-001

## Scope

Go's standard library has no color support, so add a tiny `internal/ui` package
that emits plain ANSI SGR escapes (bold, dim, red, green, yellow, cyan, reset).
Color is on only when all of these hold: the output stream is a terminal
(`os.File.Stat` reports `ModeCharDevice`), `NO_COLOR` is unset or empty,
`TERM` is not `dumb`, and `FORCE_COLOR` can turn it on for non-terminals.
Apply it to the usage text (bold headings, cyan commands and flags, dim
placeholders), to error lines (red `taskfactory: ...` prefix on stderr, judged
by stderr being a terminal), and to `validate` success (green) and failure
(red). Color decisions are made per stream. Plain text output must stay
byte-identical to today's whenever color is off.

## Constraints

No third-party modules. Do not change exit codes, messages or the order of
output, and do not color anything that scripts parse (`status` columns stay
plain unless color is off-safe). Keep the helper under about 80 lines.

## Success criteria

### C1: Color is disabled for pipes, NO_COLOR and TERM=dumb

Check: go test ./internal/ui

### C2: Piped output is byte-identical to the current plain output

Check: go test ./cmd/taskfactory

### C3: Forced color emits ANSI escapes in usage output

Check: FORCE_COLOR=1 go run ./cmd/taskfactory --help | grep -c $'\x1b\['

### C4: Repository checks pass

Check: bin/test

## Verification

Run the four checks, `bin/lint` and `go run ./cmd/taskfactory validate`. Also run
`go run ./cmd/taskfactory --help` in a real terminal and confirm it is colored,
then `NO_COLOR=1 go run ./cmd/taskfactory --help | cat -v` and confirm there are
no escape sequences. Report each exit code.

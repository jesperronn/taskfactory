# TF-093: Let bin/test select files and folders and enforce time budgets

## Goal

`bin/test` can run just the part of the suite a developer names, the way
`bin/lint` already takes `PATH...`, and it can fail when a test or the Go suite
exceeds the speed budget. Together with `--verbose` this makes a separate
`bin/check-speed` tool unnecessary.

## Dependencies

None

## Scope

Current gaps, measured on the current `bin/test`:

- `bin/test --verbose bin/` prints `Usage: bin/test [--verbose]` and exits 2.
  `bin/test` accepts no paths.
- `bin/lint` already accepts `[--autofix] [PATH...]` (`bin/lint:4-5`), so the
  two entry points differ.
- `bin/test` always runs every `bin/*.test.sh`, the Go suite, and the task
  validation. A developer who wants only `cmd/taskfactory` has to run
  `go test ./cmd/taskfactory` by hand, and that bypasses the bash suite and
  validation.
- Nothing fails when a test is slow. `--verbose` prints times, but no limit is
  enforced.

Make `bin/test` accept paths and budgets:

- `bin/test [--verbose] [--budget] [PATH...]`. Each PATH may be:
   - a `bin/*.test.sh` file, or a directory containing them (`bin/`), which runs
    only those shell tests;
   - a Go package directory (`cmd/taskfactory`, `./internal/...`), which runs
    `go test` for those packages only and skips the shell tests and task
    validation;
   - no paths: the current behavior, unchanged.
- `--budget` enforces the TF-087 limits: each shell test case must finish in
  1s, and the Go packages selected must finish in 5s in total. A breach prints
  the offending test and the time, and exits 1.
- Parse `go test -json` output for per-test timings so `--budget` can name the
  slow Go test, not only the package.
- Keep `bin/test` and `bin/test.test.sh` behavior unchanged when no arguments
  are given.

## Constraints

- Keep the no-argument behavior identical. Current exit codes stay the same:
  0 pass, 1 failure, 2 usage.
- Do not add a second test entry point. `bin/test` is the one command.
- Path arguments are checked before any test runs. A path that does not exist
  or is not a test file exits 2 with usage.
- Run `bin/lint` on the touched files and `bin/test` before reporting.

## Success criteria

### C1: A single shell test file runs alone

Check: bin/test --verbose bin/lint.test.sh

### C2: A Go package runs alone

Check: bin/test --verbose cmd/taskfactory

### C3: The budget flag fails a slow test

Check: go test ./cmd/taskfactory && bin/test.test.sh

## Verification

Run `bin/test --verbose bin/lint.test.sh`, `bin/test --verbose cmd/taskfactory`
and `bin/test --budget cmd/taskfactory`, and report each exit code. Run
`bin/test` with no arguments and report its exit code.

## Claim

Owner: claude-code
Branch: task/TF-093
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-093
Base commit: 26c3f64674ca5d44177451ea2bcf1ddb2f2466d4
Started at: 2026-10-21T09:27:34Z

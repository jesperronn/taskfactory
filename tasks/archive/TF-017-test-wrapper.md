# TF-017: Add a repository test wrapper

## Goal

Provide one `bin/test` entry point that runs the tests for every technology
currently present in TaskFactory and can be extended when another stack is
added.

## Dependencies

None

## Scope

Create executable `bin/test` and a focused `bin/test.test.sh`. Run from any
working directory by resolving the repository root from the script path. The
wrapper must work both before and after `go.mod` is introduced by TF-001. Run
every executable `bin/*.test.sh` in stable filename order; do not run `bin/test`
recursively. If `go.mod` exists, also run `go test ./...` from the root; if
absent, skip the Go check with a clear message. A future stack is added by
extending this wrapper with an explicit manifest check and test command, not by
silently assuming `go test` covers it. Return nonzero if any selected check
fails, identifying that check. Do not install dependencies, run lint, or modify
test files for other tasks. Update `AGENTS.md` and `docs/worker-instructions.md`
only if needed to keep their mandatory `bin/test` and `bin/lint` directions
accurate.

## Constraints

Keep the Bash script small and portable on macOS and Linux. Preserve unrelated
files. Do not add a framework or third-party runtime dependency. Follow
repository shell-testing conventions if available.

## Success criteria

- `bin/test` is executable and works when invoked from outside the repository.
- In a fixture without `go.mod`, it runs the discovered shell tests in stable
  order and explicitly reports that Go tests were skipped.
- In a fixture with `go.mod`, it invokes `go test ./...` from that fixture's
  root.
- A failing shell test and a failing Go test each make `bin/test` exit nonzero
  with the failing check identified; a passing fixture exits 0.
- In this repository, `bin/test`, `bin/lint`, and
  `bash -n bin/test bin/test.test.sh` pass once the wrapper is added. If the
  local `npx`/Prettier dependency prevents lint, report that blocker with its
  actual exit status rather than claiming a pass.

## Verification

Use temporary fixture repositories and a stub `go` executable to assert
invocation, order, working directory, and exit-code propagation, plus a real run
in this repository. Record exact commands and exit codes. Do not use mocked
results as evidence that real Go tests pass.

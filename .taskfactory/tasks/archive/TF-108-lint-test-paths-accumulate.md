# TF-108: bin/lint and bin/test take paths and report every problem

## Goal

`bin/lint` and `bin/test` accept any mix of files and folders, run everything
they can for the valid paths, report every problem they found, and decide the
exit code only at the very end.

## Dependencies

None

## Scope

Measured on the current scripts:

- `bin/lint bin/test bin/test.test.sh` exits 1 because shellcheck reports
  SC1091 (info) on the `source testlib.sh` line, while `bin/lint` with no
  arguments passes. The same file gives a different result alone.
- `bin/lint` stops at the first bad path: a missing path or a file kind it has
  no linter for exits 2 at once, so later valid paths are never linted.
- `bin/test` stops at the first bad path in the same way (exit 2).
- `bin/test` accepts only a `*.test.sh` file, a folder or a Go package
  pattern. A source file such as `bin/build` or `cmd/taskfactory/main.go` is
  refused even though its test is obvious.
- Passing the same test twice (a file and its folder) runs it twice.

Change `bin/lint`:

- Run shellcheck with `-x` and the script's own directory as the source
  path, so a sourced helper is followed and a single file lints the same as
  the whole project.
- Validate every path first, report every bad path (missing, or no linter for
  that kind), then still lint all the valid paths. Exit 2 after the work if any
  path was bad. Keep the usage text.
- Keep today's exit rule for lint results: mdsmith's own code when it fails,
  else 1 for any other failure, else 0, decided after every tool has run.

Change `bin/test`:

- Validate every path first and report every bad path together, with the
  usage text once. Run the valid ones. Exit 2 after the work if any path was
  bad.
- Map a source file to what covers it: a Go file in a folder selects that
  package, and `bin/<name>` selects `bin/<name>.test.sh` when it exists.
  A source file with no test is a bad path.
- Drop duplicate selections so each shell test file and each Go package runs
  once.
- Keep running every selected shell test and Go package even when an earlier
  one fails, and exit 1 only at the end.

## Constraints

Touch only `bin/lint`, `bin/lint.test.sh`, `bin/test`, `bin/test.test.sh`,
`bin/testlib.sh` and `README.md` (one short note on path arguments). With no
paths both scripts behave exactly as today. Exit codes stay 0, 1 and 2. Tests
use stubs for slow tools, never write in the real checkout, and keep every
existing assertion.

## Success criteria

### C1: A single shell file lints the same as the whole project

Check: bin/lint bin/test bin/test.test.sh

### C2: bin/lint reports every bad path and still lints the good ones

Check: bin/lint.test.sh | grep -q 'PASS.* paths: every bad path is reported'

### C3: bin/lint exits 2 only after linting the valid paths

Check: bin/lint.test.sh | grep -q 'PASS.* paths: valid paths are linted before exit 2'

### C4: bin/test reports every bad path together

Check: bin/test.test.sh | grep -q 'PASS.* paths: every bad path is reported'

### C5: bin/test maps source files to their tests

Check: bin/test.test.sh | grep -q 'PASS.* paths: source files select their tests'

### C6: bin/test runs a duplicate selection once

Check: bin/test.test.sh | grep -q 'PASS.* paths: duplicates run once'

### C7: A failing test does not stop later selected tests

Check: bin/test.test.sh | grep -q 'PASS.* paths: failures accumulate to one exit'

### C8: The whole suite and lint still pass

Check: bin/test && bin/lint

## Verification

Run each check from the repository root and report each exit code. Report the
output of `bin/lint README.md missing.md bin/test` and
`bin/test missing bin/build.test.sh cmd/taskfactory` before and after.

## Claim

Owner: sonnet-worker-108
Branch: task/TF-108
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-108
Base commit: 0f45d202fa3fc9fde79243c573e2901be5ebbae1
Started at: 2026-10-21T22:12:21Z

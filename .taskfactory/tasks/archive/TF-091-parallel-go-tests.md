# TF-091: Run independent Go tests in parallel

## Goal

Independent Go tests and subtests run with `t.Parallel()`, so the wall time of
`go test -count=1 ./...` falls toward its slowest package instead of the sum
of its tests, with no test lost and no data race.

## Dependencies

None

## Scope

Fact base measured on 2026-10-21 on an 18 CPU machine. No `*_test.go` file
calls `t.Parallel()` today. The top-level test times sum to about 259s, and
`go test ./...` takes about 68s wall under load. The time is process spawns:
`git init` costs about 25ms, `git commit` about 38ms, and one fixture runs 12
to 15 `git` processes. The CLI binary is already built once per package by
`TestMain` and `sync.Once` in `cmd/taskfactory/main_test.go`, so tests do not
rebuild it. `t.Parallel()` is the largest win and does not depend on TF-088,
TF-089 or TF-090, which cut the per-test cost and merge cleanly with it.

Add `t.Parallel()` to every top-level test and subtest that shares no
process-global state, in this package order, measuring after each package:

1. `cmd/taskfactory` (largest, about 40s alone, target 5 to 8s)
2. `internal/integrate`
3. `internal/requeue`
4. `internal/promote`
5. `internal/fail`
6. `internal/verify`
7. `internal/work`
8. `internal/claim`

A test must stay serial when it:

- calls `t.Chdir`, `os.Chdir`, `t.Setenv` or `os.Setenv`, because both are
  process-global and Go panics on `t.Setenv` in a parallel test;
- registers into or reads the shared registry, `paths.Register`;
- uses a shared fixture, template or binary it would modify, or writes a fixed
  path outside `t.TempDir()`;
- changes package-level variables, such as a stubbed clock or runner.

The CLI binary and any template from TF-088 are read-only for tests. For
table tests, call `t.Parallel()` in the subtest and copy the loop variable
only if the module's Go version needs it. Keep slow serial tests few and name
each one with a short comment saying which rule keeps it serial. Use
`go test -p` only as a last step, if package parallelism is not enough.

Realistic target: Go wall time for the full suite of about 10 to 15s on a
loaded machine after this task alone, and lower once the siblings land.

## Constraints

- Do not add `t.Parallel()` to a test that touches process-global state.
- Do not delete, skip or weaken a test, and do not change what it asserts.
- Do not edit TF-089 or TF-090 files except where a rebase needs it.
- Run `gofmt`, `go vet` and `bin/lint` on the touched files.

## Success criteria

### C1: Race detector is clean

Check: go test -race -count=1 ./...

### C2: The tests still pass uncached

Check: go test -count=1 ./...

### C3: No test was lost

Check: go test -count=1 -json ./... | grep -c '"Action":"run"'

### C4: The slowest package is far below its old time

Check: go test -count=1 -timeout 30s ./cmd/taskfactory

### C5: The whole task tree validates

Check: go run ./cmd/taskfactory validate .taskfactory/tasks

## Verification

Before editing, record the C3 count and per-package wall times from
`go test -count=1 ./...`. After editing, run every check and report each exit
code. Report the C3 count before and after; the two numbers must be equal, and
a name present before and absent after is a failure. Report per-package times
before and after, then run `bin/test` and `bin/lint` and report their codes.

## Claim

Owner: sonnet-worker-091
Branch: task/TF-091
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-091
Base commit: ef7c93b68b52e716bf0e587fd58f07a36c4d2fd7
Started at: 2026-10-21T20:50:31Z

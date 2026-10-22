# TF-109: Stream worker output during taskfactory work

## Goal

Show worker output on the terminal while a `taskfactory work` run is still
going, not only after the worker exits. The log receives the same lines as they
arrive.

## Dependencies

- TF-056

## Scope

Change `internal/work/work.go`, which today collects the adapter output and
writes it to the log only after the worker finishes (see the `log` and
`----- adapter output -----` writes near lines 114 to 136). Stream adapter
output through an `io.Writer` that tees to the terminal and the log file.
Touch the adapter packages under `internal/adapter/` only if they must expose
a streaming hook. The stream-json and rpc modes deferred by TF-056 are out of
scope.

## Constraints

The final work log keeps the same layout and header as today. Worker
selection and retry behaviour do not change. Use only the Go standard library.

## Success criteria

### C1: A named test proves output streams before exit

Check: go test ./internal/work -run TestStreamsWorkerOutputBeforeExit

### C2: The whole test suite passes

Check: bin/test

### C3: Lint passes on the repository

Check: bin/lint

## Verification

Run each check from the repository root and report each exit code. Report
whether the test in C1 was added by the worker and that it fails before the
change.

## Claim

Owner: haiku-worker-109
Branch: task/TF-109
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-109
Base commit: 7c244f08c87029f4b1f3febab96dd7988d27bbe7
Started at: 2026-10-22T12:36:03Z

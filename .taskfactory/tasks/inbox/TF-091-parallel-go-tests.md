# TF-091: Run independent Go tests in parallel

## Goal

Independent Go tests run with `t.Parallel()`, so the wall time of
`go test -count=1 ./...` falls toward the length of its slowest package, not the
sum of its tests.

## Dependencies

- TF-088
- TF-089
- TF-090

## Scope

After the fixture and wait fixes land, mark tests that share no state as
parallel. Check each candidate for:

- `os.Chdir`, `os.Setenv` or package-level mutable state (these must stay
  serial);
- writes to the same fixed path under `/tmp`;
- a shared package-level template or binary (the template from TF-088 and the
  CLI binary from `cmd/taskfactory`'s `TestMain` must remain read-only).

Start with `cmd/taskfactory`, `internal/integrate` and `internal/requeue`,
which are the largest packages (about 38s, 41s and 30s before the fixes). Use
`go test -p` only as a last step, if package-level parallelism alone is not
enough.

## Constraints

- Do not add `t.Parallel()` to a test that touches process-global state.
- Do not reduce test coverage.
- Run `gofmt`, `go vet` and `bin/lint` on the touched files.

## Success criteria

### C1: Go suite meets the time limit

Check: bin/test --budget ./...

### C2: Race detector is clean

Check: go test -race -count=1 ./...

## Verification

Run `bin/test --budget ./...`, `go test -race -count=1 ./...`, and report both exit
codes and the wall time.

# TF-089: Stop the adapter stall tests from waiting out the kill grace

## Goal

The adapter stall tests finish in well under a second each instead of about
3s, without changing what they assert about timeouts.

## Dependencies

- TF-093

## Scope

Each stall test runs a fake binary that sleeps 5s and sets a 1s adapter
timeout. The timeout kills the process, but the orphaned `sleep` child keeps
the captured stdout pipe open. `cmd.WaitDelay = KillGrace` in
`internal/adapter/common/common.go:149` then waits the full 2s `KillGrace`
(`common.go:41`) before returning. Measured per test: 3.0s, against a 1s
timeout.

Affected tests:

- `internal/adapter/common/common_test.go:96` (`timeout is stalled`) and
  `TestLaunchMapsExitStallAndBlocked` at about 3.0s each.
- `internal/adapter/claude/stall_test.go:15` (`/bin/sleep 5`) at about 3.0s.
- `internal/adapter/omp/stall_test.go:15` and `internal/adapter/pi/run_test.go:47`
  at about 3.1s each.

Make the fake binary exit with the process group when it is killed (for
example `exec /bin/sleep 5`, so no child outlives the timeout), or give the
test a shorter `WaitDelay` through an unexported package variable set only by
tests. Pick the option that keeps the production `KillGrace` and the timeout
behavior unchanged.

Also check `internal/claim/claim_test.go:58` and `:79`: its `waitForFile` and
`assertFileRemainsAbsent` helpers poll with real timers that should be
replaced with a short fixed poll interval.

## Constraints

- Do not change the production timeout or the 2s `KillGrace` value.
- Do not delete or weaken a stall test. The stall must still be detected and
  reported as stalled.
- Run `gofmt`, `go vet` and `bin/lint` on the touched files.

## Success criteria

### C1: Stall tests finish under a second

Check: go test -count=1 -run 'Stall|TestLaunchMapsExitStallAndBlocked' ./internal/adapter/...

### C2: Whole Go suite meets the time limit

Check: bin/test --budget ./...

## Verification

Run the stall tests with `-v` and report each test's duration. Run
`bin/test --budget ./...` and report its exit code.

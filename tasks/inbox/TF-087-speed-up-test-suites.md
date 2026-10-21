# TF-087: Speed up the bash and Go test suites

## Goal

The bash tests each finish in under 1 second and `go test -count=1 ./...`
finishes in under 5 seconds, without losing coverage.

## Dependencies

None

## Scope

Iterative: measure, fix the slowest item, remeasure, repeat until the user
is satisfied. Baseline on 2026-10-21 (uncached, one machine):

- Go: 84s wall, 99s sys vs 64s user. `cmd/taskfactory` 84s, `integrate` 37s,
  `requeue` 27s, `verify` 10s, `promote` 9s, `fail` 9s, `work` 9s.
- Bash: `test.test.sh` 144s, `release.test.sh` 26s, `lint.test.sh` 17s,
  `build.test.sh` 13s, `package.test.sh` 13s, `install.test.sh` 11s,
  `release.version.test.sh` 5.5s, `release-publish.test.sh` 5.1s,
  `lint.task.test.sh` 4.5s, `lint.real-mdsmith.test.sh` 2.9s.
- High sys time points at process spawning: repeated `git init`, `go build`
  and `go run` per test. Likely fixes: build the binary once per package
  (`TestMain`) and share it, share one prepared git template repo copied per
  test, run independent tests with `t.Parallel()`, and stop `bin/test.test.sh`
  re-running the whole suite.
- Limits are enforced by `bin/test --budget`, planned in TF-093: 5s total for
  the selected Go packages and 1s for each bash test case. `bin/check-speed` is
  dropped.
- Per-test timing for bash tests is in place: every `bin/*.test.sh` accepts
  `--verbose` (commit 3d00086).

## Constraints

Do not delete or weaken a test to make it faster. Keep each test isolated.

## Success criteria

### C1: Go suite is fast

Check: bin/test --budget ./...

### C2: Every bash test is fast

Check: bin/test --budget bin/

## Verification

Run both checks and report the timings and exit codes.

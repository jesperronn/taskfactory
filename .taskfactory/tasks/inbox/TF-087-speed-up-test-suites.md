# TF-087: Speed up the bash and Go test suites

## Goal

The test suites run fast enough to use on every change, with honest budgets:
the full Go suite in about 10 to 12s on an idle machine, any selected Go
package set in under 5s, and each stubbed bash case in under 1s, without
losing coverage.

## Dependencies

None

## Scope

This is the umbrella. It owns the numbers and delegates the work to children.
Facts measured on 2026-10-21 on an 18 CPU machine, replacing the earlier text:

- Go: no `*_test.go` uses `t.Parallel()`. Top-level test time sums to about
  259s and `go test ./...` takes about 68s wall under load.
- The cost is process spawns: `git init` about 25ms, `git commit` about 38ms,
  and one fixture runs 12 to 15 `git` processes.
- The CLI binary is already built once per package by `TestMain` and
  `sync.Once` in `cmd/taskfactory/main_test.go`. There is no per-test
  `go build` or `go run`, and no `time.Sleep` in Go tests.
- Stall tests cost about 3.0s each (TF-089). Dry-run tests rebuild fixtures
  per row (TF-090).
- `bin/test` takes paths and `--budget` and parses `go test -json` (TF-093,
  landed). `bin/check-speed` is dropped.
- `bin/test.test.sh` took 144s in the first baseline because it re-ran the
  suite. After TF-093 it runs stubbed children and takes about 3s.
- Bash timings per case come from `--verbose` on every `bin/*.test.sh`.

Honest budgets. Go compile and link cost 2 to 3s, `cmd/taskfactory` spawns
the CLI hundreds of times and the stall timeout has a 1s floor, so 5s for
`go test ./...` is not safely attainable. The target is 10 to 12s idle for
the full suite and 10 to 15s loaded. The 5s limit applies to a selected
package set through `bin/test --budget <pkg>`. One second per shell case holds
only for stubbed cases; cases that run real `git`, `go` or `mdsmith` are
excluded from that limit and named in the child that owns them.

Children, in no required order except where a child says so:

- TF-088: share a git template in Go fixtures, including `cmd/taskfactory`.
- TF-089: remove the stall kill grace wait.
- TF-090: cheap dry-run snapshots.
- TF-091: `t.Parallel()` for independent Go tests, the largest win.
- TF-092: cut bash test process spawns.
- TF-106: keep `bin/test.test.sh` cheap with a guard (the follow-up to
  TF-093 that owns the wrapper's own test).

Close this task when the children are archived and the checks below pass.
Re-measure at that point and adjust the budgets in the child tasks rather than
weakening a test.

## Constraints

Do not delete or weaken a test to make it faster. Keep each test isolated.
Do not edit children from this task; open a new task for a gap.

## Success criteria

### C1: The full Go suite meets the idle budget

Check: go test -count=1 ./...

### C2: A selected Go package stays under the 5s budget

Check: bin/test --budget ./internal/promote

### C3: Stubbed bash tests are fast

Check: bin/test --budget bin/test.test.sh

### C4: The whole task tree validates

Check: go run ./cmd/taskfactory validate .taskfactory/tasks

## Verification

Run each check, report the exit codes, and report the wall time of C1 on an
otherwise idle machine and the per-package times from `bin/test --budget`.

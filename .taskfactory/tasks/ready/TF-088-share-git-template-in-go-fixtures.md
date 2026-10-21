# TF-088: Share a prepared git template across Go test fixtures

## Goal

Go test fixtures stop running `git init`, `git config` and an initial commit
per test. Every affected test still starts from a fresh, isolated repository,
and `go test -count=1 ./...` gets faster by the time those git calls cost.

## Dependencies

- TF-093

## Scope

Most Go package tests build a real git repository per test by running many
`git` subprocesses:

- `internal/integrate/integrate_test.go:350` (`newFixtureWithGitignore`) runs
  `git init`, `config`, `add`, `commit`, `branch` and worktree setup per test.
  `TestCheckMainMalformedStopFailsClosedUnchanged` takes about 5.6s.
- `internal/requeue/requeue_test.go:127` and `cleanup_test.go` run real
  worktree and branch commands per test (`TestCleanupOutsideRoot` about 3.8s).
- `internal/verify/verify_test.go:99`, `:342`, `internal/promote/promote_test.go:55`,
  `internal/fail/fail_test.go:141`, `internal/claim/operation_test.go:266,273`,
  `internal/workprompt/workprompt_test.go:40`, `internal/work/fixture_test.go:39`
  each have the same pattern.
- `cmd/taskfactory/main_test.go:655` (`initGitProject`) and `stateFixture`
  run the same `git init`, `config` and commit sequence per test, hundreds of
  times, and are in scope as well. The CLI binary built there stays shared.

Build one template repository per package in `TestMain` (or a `sync.Once`
helper) with the config, task files and initial commit already made. Each test
copies it with `cp -c` on APFS where available, or a plain recursive copy, and
then makes only the commit or branch it needs. Keep the template outside every
test's temp directory and delete it in `TestMain`. The template must contain
no worktrees, and no claim line or other text with an absolute path of the
template itself.

Do not change what the tests assert. Only move setup that every test repeats.

## Constraints

- Do not delete or weaken a test to make it faster.
- Keep each test isolated: no test may depend on another test's repository or
  on a shared mutable branch.
- Do not add test-only branches to production code.
- Run `gofmt`, `go vet` and `bin/lint` on the touched files.

## Success criteria

### C1: The Go suite meets the time limit

Check: bin/test --budget ./internal/integrate

### C2: Fixture behavior is unchanged

Check: go test -count=1 ./internal/...

## Verification

Run `bin/test --budget ./internal/integrate`, `go test -count=1 ./...`, and
report the per-package timing before and after, plus exit codes. Run
`bin/test` and report its exit code.

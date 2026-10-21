# TF-090: Make dry-run snapshots cheap in cmd/taskfactory tests

## Goal

The `--dry-run` tests in `cmd/taskfactory` assert the same "changes nothing"
guarantee with fewer `git` and filesystem calls, so the dry-run tests drop out
of the top of the timing list.

## Dependencies

- TF-087

## Scope

`cmd/taskfactory/dryrun_test.go:84` (`snapshot`) runs `git` commands and
hashes the whole tree before and after each command. It is called from
`runDryScenario` and from `TestDryRunLeavesGitStatusAndWorktreesUnchanged`,
which takes about 6.2s. Other dry-run tests take 2 to 5s each:
`TestDryRunOutputFormat` (5.3s), `TestDryRunIntegrateCheckMainChangeNothing`
(5.1s), `TestDryRunFailRequeueChangeNothing` (3.2s) and
`TestDryRunInitPromoteClaimChangeNothing` (2.9s).

Keep the snapshot as the assertion, but:

- take one git snapshot (`status`, `worktree list`, refs) per side, not one per
  git subcommand, by running a single `git` call that prints them together
  where possible;
- hash only the files a scenario can touch, not the whole tree;
- reuse the state fixture built by `stateFixture` across the subtests of one
  scenario table, rather than rebuilding it for each row, if the rows only read it.

## Constraints

- Keep every "changes nothing" assertion. Do not weaken the comparison, and do
  not skip the scenario that changes state.
- Do not change dry-run output text.
- Run `gofmt`, `go vet` and `bin/lint` on the touched files.

## Success criteria

### C1: Dry-run tests are fast

Check: go test -count=1 -run 'TestDryRun' ./cmd/taskfactory

### C2: Whole Go suite meets the time limit

Check: bin/check-speed go

## Verification

Run the dry-run tests with `-v` and report each test's duration before and
after. Run `bin/check-speed go` and report its exit code.

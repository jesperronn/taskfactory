# TF-090: Make dry-run snapshots cheap in cmd/taskfactory tests

## Goal

The `--dry-run` tests in `cmd/taskfactory` assert the same "changes nothing"
guarantee with fewer `git` and filesystem calls, so the dry-run tests drop out
of the top of the timing list.

## Dependencies

- TF-093

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

### C2: Dry-run tests finish within 20 seconds in total

Check: go test -count=1 -timeout 20s -run 'TestDryRun' ./cmd/taskfactory

## Verification

Run the dry-run tests with `-v` and report each test's duration before and
after. Run the C2 command and report its exit code.

## Claim

Owner: sonnet-worker-090
Branch: task/TF-090
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-090
Base commit: 2954270410742b2dcf763978b7c15e2bf6ae41a8
Started at: 2026-10-21T20:31:44Z

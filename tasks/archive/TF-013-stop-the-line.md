# TF-013: Recheck and recover stopped main

## Goal

Keep ordinary integration stopped after failed main verification until the
current main commit passes its required checks.

## Dependencies

- TF-012

## Scope

Extend TF-012's `.taskfactory/integration-stop.json` record and fail-closed
reject behavior without changing its path or key schema. Add
`taskfactory check-main` under the integration lock. Read and validate the
canonical stop file or a lone `.tmp` sibling. A malformed record fails closed
and remains for manual inspection. Run configured `verification.main` commands
on the current local main checkout in order. On failure, atomically write or
update the canonical stop record with the current main commit, failing command,
exit code or start error, and output. Clear the canonical record and any valid
leftover temp only after all checks pass on the current main commit. If a stop
exists but no main checks are configured, fail closed with an actionable error.
Do not clear a stop merely because main moved. Isolated workers may continue;
repair-task prioritization stays with the orchestrator.

## Constraints

Reuse the stop schema and lock from TF-012. Do not reset main, fetch or push,
archive a failed task, or silently remove malformed stop data. Use direct Git
subprocesses and Go's standard library.

## Success criteria

### C1: Failed main checks persist exact stop data and block later integration

Check: go test ./internal/integrate

### C2: A failed recheck updates the stop and retains it

Check: go test ./internal/integrate

### C3: Passing checks on current main clear valid stop state and allow integration

Check: go test ./internal/integrate

### C4: Lone valid temp and malformed stop records behave as specified

Check: go test ./internal/integrate

### C5: Standard repository checks pass

Check: bin/test

## Verification

Use temporary Git repositories and real configured pass/fail commands. Cover a
post-merge failure, rejected integration, failed and successful rechecks, main
moving between checks, absent main-check config, and canonical/temp malformed
records. Compare exact stop bytes before and after failure paths. Run
`bin/test`, `bin/lint`, `go vet ./...`, and whole-tree task validation; report
exact exits.

# TF-013: Stop integration when main is broken

## Goal

Prevent ordinary integration while main fails its required checks and retain
repair evidence.

## Dependencies

TF-012 must be archived before promotion to ready.

## Scope

Persist an explicit stopped integration state under `.taskfactory/` when main
verification fails, including a post-merge failure. The state records the exact
main commit and failing command. Reject ordinary `integrate` while stopped;
allow isolated workers to continue. Add `taskfactory check-main` to rerun
configured main verification on the current main commit. Clear the stopped state
only after all checks pass; do not clear it merely because main moved. Keep
repair-task prioritization with the orchestrator.

## Success criteria

- Failed main checks stop subsequent ordinary integrations without moving their
  task files.
- The stop record names the failing command, exit code, main commit, and
  relevant output.
- A failed recheck retains the stop; a passing `taskfactory check-main` clears
  it and permits subsequent ordinary integration.
- Post-merge failure never archives its task as successful.
- `go test ./...` and `go vet ./...` pass.

## Verification

Exercise red-main, post-merge failure, rejected integration, and successful
recovery in temporary Git repositories.

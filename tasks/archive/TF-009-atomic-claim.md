# TF-009: Guard claim eligibility atomically

## Goal

Provide the local transaction guard and eligibility checks that TF-010 will use
to claim one ready task exactly once.

## Dependencies

- TF-002
- TF-003

## Scope

Add an internal claim package with
`WithClaimLock(projectRoot string, fn func() error) error` and an eligibility
check for a ready task ID. Use a project-local OS advisory lock at
`.taskfactory/claim.lock` that serializes separate processes and releases when a
process exits. Hold the lock across the eligibility check and the caller's
eventual branch/worktree setup and ready-to-active move. Eligibility requires a
valid ready contract, all listed dependencies in archive, and fewer active task
files than `workers.max_parallel`. Return actionable errors that identify the
task and failed condition. TF-010 calls these operations and performs the actual
protocol-level claim under the lock.

## Constraints

Keep this operation internal: do not add a CLI command, move task files, create
branches or worktrees, or write incomplete Claim metadata. Do not introduce a
new runtime dependency. Support the initial macOS and Linux platforms.

## Success criteria

### C1: Independent processes cannot hold the claim lock concurrently

Check: go test ./...

### C2: Missing or unarchived dependencies and full worker capacity reject eligibility

Check: go test ./...

### C3: Eligible ready tasks pass without changing task or Git state

Check: go test ./...

### C4: Standard repository checks pass

Check: bin/test

## Verification

Use temporary project trees and at least one subprocess concurrency test that
proves the lock serializes separate processes. Test boundary capacity values,
unknown task IDs, missing and unarchived dependencies, and release on callback
error. Compare task bytes and paths before and after every eligibility test. Run
`bin/test`, `bin/lint`, `go vet ./...`, and whole-tree validation; report exact
exits.

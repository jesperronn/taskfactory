# TF-011: Record worker verification evidence

## Goal

Run configured verification and retain a reviewable PASS, FAILED, or BLOCKED
result.

## Dependencies

TF-002, TF-003, TF-010, and TF-021 must be archived before promotion to ready.

## Scope

Implement `taskfactory verify <ID>` for an active worktree using the command
execution rules in `docs/config-v1.md`. Store append-only attempt records beside
the active task using the evidence path and field names defined by
`docs/task-format-v1.md`. Record task ID, branch/base/result commits, changed
files, commands, exit codes, attempts, and relevant output. Preserve failed
attempts so a worker can diagnose and repair. Each invocation runs the full
configured worker command list in order and records the first failure; only an
all-pass attempt has PASS status. Do not mark a task complete on a failed check.
Keep the effort/retry budget under the orchestrator's control; do not build an
agent loop.

## Success criteria

- Passing and failing commands produce accurate exit codes and retained
  evidence.
- A failed attempt followed by a pass retains both attempts in order; the latest
  attempt determines current verification status.
- Missing worktree or malformed active metadata fails clearly.
- `go test ./...` and `go vet ./...` pass.

## Verification

Use a temporary Git project and deterministic pass/fail commands. Read the saved
evidence and compare it with actual command results.

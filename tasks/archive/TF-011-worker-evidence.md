# TF-011: Record worker verification evidence

## Goal

Run task criterion checks and configured worker verification, then retain a
reviewable PASS, FAILED, or BLOCKED result for every recorded attempt.

## Dependencies

- TF-002
- TF-003
- TF-010
- TF-021

## Scope

Implement `taskfactory verify <ID>` for an active task and its claimed worktree.
Run criterion checks in task order, then `verification.worker` commands in
config order, using the execution rules in `docs/config-v1.md`. Stop at the
first non-zero exit or shell start error. Append one JSONL record per attempt
using the exact schema and locking rules in `docs/task-format-v1.md`. Record
command source, criterion, exact command, exit code, combined output, start
error, and changed files. Do not mark a task complete on a failed or blocked
check. Keep retries under orchestrator control; do not build an agent loop.

## Constraints

Use the existing config, task, and Claim contracts without changing task
lifecycle state. Use a per-task OS lock to serialize evidence appends across
processes. Preserve previous evidence bytes exactly and add no new runtime
dependency.

## Success criteria

### C1: Verification runs the combined sequence in order and stops at the first failure

Check: go test ./...

### C2: Evidence JSONL records exact command outcomes and changed files

Check: go test ./...

### C3: Attempts append in order and concurrent verifies get distinct numbers without changing prior bytes

Check: go test ./...

### C4: Missing worktree or malformed active metadata fails clearly

Check: go test ./...

### C5: Malformed or logically inconsistent prior JSONL fails without changing any evidence bytes

Check: go test ./...

### C6: Shell start errors record BLOCKED and command failures record FAILED

Check: go test ./...

### C7: The repository checks pass

Check: bin/test

## Verification

Use a temporary Git project and deterministic task/config commands to verify
execution order, stop behavior, recorded output and exit codes, and PASS,
FAILED, and BLOCKED outcomes. Compare changed files with the worktree snapshot.
Run two verifies against the same task concurrently; confirm distinct
consecutive attempt numbers and byte-for-byte preservation of earlier JSONL
lines. Confirm malformed JSON and valid JSON with a wrong task ID, unknown or
missing key, wrong field type, duplicate attempt, or attempt gap are rejected
without modifying any evidence bytes. Verify a shell start failure produces a
BLOCKED record with null exit code, and a non-zero command produces FAILED. Run
`bin/test`, `bin/lint`, `go vet ./...`, and the whole-tree validator; report
exact exits.

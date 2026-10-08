# TF-028: Worker state machine preserving Git state and evidence

## Goal

Handle completion, failure, stall, stop, and resume so that each preserves the
task's Git state (branch and worktree) and its evidence.

## Dependencies

- TF-010
- TF-011

## Scope

Implement the worker state machine that keeps the claimed branch, worktree, and
base commit intact across states. Completion records a result commit and a PASS;
failure records FAILED; a stall records `stalled` without archiving or
completing; a stop records the point reached; a resume reuses the same branch,
worktree, and base commit and appends a new attempt number. Hand failed work off
for repair or re-assignment without losing evidence.

## Constraints

Do not silently complete, archive, or discard work in any state. Preserve prior
evidence bytes exactly. Use the existing claim and evidence contracts without
inventing new lifecycle states.

## Success criteria

### C1: Completion, failure, stall, stop, and resume each preserve the branch, worktree, and evidence

Check: go test ./...

### C2: A stalled or failed run is recorded and left in place, never archived or completed

Check: go test ./...

### C3: The repository checks pass

Check: bin/test

## Verification

Use a temporary Git project to exercise each state: a completed run with a
result commit, a failed check, a stalled run that exceeds its timeout, a stopped
run, and a resume that appends a new attempt number against the same base
commit. Confirm branch, worktree path, and evidence bytes are preserved and that
stall/failure never archive or complete the task. Run `bin/test`, `bin/lint`,
and the whole-tree validator; report exact exits.

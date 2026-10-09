# TF-028: Worker state machine preserving Git state and evidence

Promotion blocked: `docs/local-workers-v1.md` requires a stall to be recorded as
`stalled`, but the evidence `outcome` enum in `docs/task-format-v1.md` allows
only `PASS`, `FAILED`, and `BLOCKED`. The owner must decide how a stall is
recorded before this can be promoted. The CLI also has no stop or resume command
yet; inbox TF-033 may overlap with this scope.

Planner note: the owner has deferred the stall mapping decision. The expected
mapping is that a stalled or stopped run is recorded with
`taskfactory fail <ID> --outcome BLOCKED` (inbox TF-048). That keeps the Claim
block and evidence in place and uses BLOCKED, which is already an evidence
outcome, so no `stalled` outcome is needed. This task stays in inbox until the
owner confirms the mapping.

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

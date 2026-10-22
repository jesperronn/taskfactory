# TF-112: Retry budget counts across requeues

## Goal

The retry budget from TF-110 caps attempts per requeue cycle only. `requeue`
renames the evidence file aside when it resets the counter, so a task that is
requeued after every failed cycle is never refused. Make the budget count the
total attempts of a task across requeues so a task cannot be requeued forever.

## Dependencies

- TF-110

## Scope

- Keep a persistent per-task attempt count that survives the evidence file
  being renamed aside. Decide the place (for example the aside files or a
  small counter file) and record the choice in `docs/config-v1.md`.
- `requeue` refuses when the total count reaches `retry_budget`, with the
  same message shape as today and the total count in it.
- Add a test that requeues a task across several cycles, each below the
  budget on its own, and shows the refusal once the total reaches the budget.

## Constraints

Do not change the default budget or the config key name. Existing evidence
files and their format stay readable. No change to commit signing.

## Success criteria

### C1: The cross-cycle refusal is tested

Check: go test ./internal/requeue -run TestRequeueBudgetCountsAcrossCycles

### C2: The full test suite passes

Check: bin/test

### C3: Lint passes

Check: bin/lint

## Verification

Run each check from the repository root and report each exit code.

## Claim

Owner: haiku-worker-112
Branch: task/TF-112
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-112
Base commit: f436d2ebf313e100fe0c80ca1a5885030f9c8762
Started at: 2026-10-22T12:42:26Z

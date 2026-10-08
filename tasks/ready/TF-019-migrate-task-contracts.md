# TF-019: Migrate task contracts to the v1 format

## Goal

Make the repository task tree consistent with the state-specific contract that
TF-003 will validate.

## Dependencies

- TF-006

## Scope

Clarify `docs/task-format-v1.md` for inbox, ready, active, failed, and archived
files. Preserve the protocol's rule that inbox stories may be incomplete; define
the minimum identity and structural checks for them. Require the full executable
contract on promotion to ready. Define how existing archived records with
missing claim data are accepted without inventing history. Migrate current task
files to the appropriate state contract while preserving their intent, IDs,
dependencies, and lifecycle locations. Update TF-003's implementation contract
if clarification changes its required behavior.

## Constraints

Do not implement the validator or move tasks between state directories. Do not
fabricate claims, check results, or completion evidence. Keep the grammar small
and directly parseable by Go's standard library.

## Success criteria

### C1: Each lifecycle state has an unambiguous validation rule

Check: bin/lint

### C2: Existing task files conform to their documented state rules

Check: bin/test

### C3: No task identity or lifecycle state is changed by migration

Check: git diff --check

## Verification

Inspect every task file against the revised contract and compare file paths and
IDs before and after. Run `bin/test`, `bin/lint`, and `git diff --check`; report
their exit codes and any manual checks separately.

# TF-033: Task lifecycle commands

## Goal

Close the gap between the protocol roles and the CLI for planner and
orchestrator transitions. Only `requeue` remains.

## Dependencies

- TF-012
- TF-013
- TF-049

## Scope

Delivered:

- Promote inbox to ready, only if the ready contract validates: TF-047
  (`internal/promote`, `TestRefusesInvalidContractWrongStateAndUnknownID`).
- Mark active work failed or blocked with retained evidence: TF-048
  (`internal/fail`, `TestRetainsTaskAndEvidenceBytes`).
- Atomic, whole-tree validation, commit of only the task moves: both, see
  `TestIsolationRefusesStagedIndexAndCommitsOnlyTaskPaths` and
  `TestRefusesWhenValidationFailsAndRestoresFile`.

Remaining: requeue a failed task. TF-049 (inbox, depends on TF-047 and
TF-048) specifies `taskfactory requeue`. TF-033 is done when TF-049 is
archived, then this file can be archived with it. The README still says "There
is no `requeue`".

## Success criteria

- Each transition is a documented command with refusal tests for invalid
  states. Done for `promote` and `fail`; open for `requeue` (TF-049).
- `go run ./cmd/taskfactory validate` passes after every transition. Done for
  `promote` and `fail`; open for `requeue` (TF-049 C8).
- No command touches unrelated files. Done for `promote` and `fail`; open for
  `requeue`.

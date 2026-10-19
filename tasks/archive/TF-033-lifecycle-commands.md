# TF-033: Task lifecycle commands

Closed: every scope item is delivered. The last item, `requeue`, was delivered
by TF-049 (archived; `internal/requeue`, `cmd/taskfactory/requeue_test.go`).
Evidence: `taskfactory requeue --help` exits 0 and prints the claimed-work rule
and the counter reset; `TestRequeuesRestoresEverythingWhenTreeValidationFails`
covers whole-tree validation; `TestRequeueUsageErrorsExitTwoAndHelpExitsZero`
covers exit codes; the README quick start documents the requeue branch.

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

Requeue a failed task: TF-049 (archived). Remaining: none.

## Success criteria

- Each transition is a documented command with refusal tests for invalid
  states. Done for `promote` (TF-047), `fail` (TF-048) and `requeue` (TF-049).
- `go run ./cmd/taskfactory validate` passes after every transition. Done for
  all three; `requeue` runs it before committing (TF-049 C8).
- No command touches unrelated files. Done for all three; requeue stages only
  the removed failed path and the added target path.

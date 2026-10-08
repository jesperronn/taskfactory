# TF-049: Requeue command for failed work

## Goal

Let the orchestrator return failed work to a ready state so another worker can
retry it, without hand-moving files.

## Dependencies

- TF-048

## Scope

Add `taskfactory requeue <ID>` for tasks in `tasks/failed/` that have never
been claimed, moving them to `tasks/ready/` after the ready contract validates.
Behaviour matches `promote` (TF-047): validate, move, validate the tree,
commit only the two task paths, exit 0, 1 or 2.

## Constraints

The Claim block is retained unchanged on failed work, but a ready task must not
have one. A requeue of a claimed task therefore cannot move to ready without
rewriting history, which the protocol forbids. Until the open questions below
are answered, refuse any failed task that has a Claim block.

## Success criteria

### C1: Never-claimed failed task requeues to ready and commits

Check: go test ./internal/requeue -run Requeues

### C2: Claimed failed task is refused unchanged

Check: go test ./internal/requeue -run RefusesClaimed

### C3: Whole task tree validates after requeue

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run the three checks and `bin/test` and `bin/lint`; report each exit code.

## Open questions

- Who may requeue: the planner, the orchestrator, or either? The docs say the
  orchestrator decides retry, re-plan, or abandon, but do not name a command
  owner.
- Should a claimed failed task requeue to `tasks/inbox/` (where Claim is not
  rejected by validation) or be re-planned by the planner first? The docs do
  not say whether a Claim block may be stripped on requeue; the protocol says
  Claim is retained unchanged.
- Should a failed task with evidence be requeued to ready at all, or only to
  inbox for re-planning?
- Does requeue need a new attempt-count reset, or does the evidence attempt
  counter continue? The protocol says attempts only increase.

# TF-068: Local worker delegation remainder

## Goal

Finish the parts of local worker delegation that TF-016 did not deliver: live
progress during a run, retry budgets, and, as a post-v1 idea, evidence-based
automatic worker selection.

## Dependencies

- TF-056
- TF-049
- TF-048

## Scope

Three independent items. Split into separate tasks when one is promoted.

- Live progress: `taskfactory work` writes its log only when the run ends.
  Stream worker output to the terminal and the log while the run is going
  (the `stream-json` and rpc modes were deferred in the TF-056 notes). Done
  when an operator sees output during a run and the final log is unchanged.
- Retry budgets: retries are manual `fail` then `requeue` with no limit.
  Add a configurable limit on repeated attempts per task. Done when a task
  past its budget is refused by `requeue` (or flagged) with a clear message,
  and the limit is covered by a test.
- Evidence-based automatic worker selection (exploration, post-v1 idea):
  selection stays manual in v1. Needs data from many runs (adapter, model,
  outcome, attempts, duration) before any rule can be justified. Done when
  a written exploration says which recorded data would justify automatic
  selection and whether to build it; no code is required.

## Notes

Remainder of TF-016 (archived), which delivered everything else in local
worker delegation. Unrefined backlog item: not promoted, no success criteria
yet. Selection of the worker remains the operator's choice in v1.

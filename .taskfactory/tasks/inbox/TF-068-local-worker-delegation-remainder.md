# TF-068: Local worker delegation remainder

## Goal

Finish the parts of local worker delegation that TF-016 did not deliver: live
progress during a run, retry budgets, and an exploration of evidence-based
automatic worker selection.

## Dependencies

- TF-048
- TF-049
- TF-056

## Scope

Three items. Each is delivered on its own. Split into separate tasks when one
is promoted.

- Live progress: `taskfactory work` streams worker output to the terminal and
  to the log while the run is going. The stream-json and rpc modes deferred in
  TF-056 are out of scope.
- Retry budgets: add a configurable limit on repeated attempts per task.
  `requeue` refuses a task past its budget with a clear message.
- Evidence-based automatic worker selection: a written exploration of which
  recorded data (adapter, model, outcome, attempts, duration) would justify
  automatic selection, and whether to build it. No code.

## Constraints

Worker selection stays manual in v1. The final work log for a run stays the
same as today. Each code item has focused test coverage.

## Success criteria

### C1: The whole test suite passes

Check: bin/test

### C2: Lint passes on the repository

Check: bin/lint

### C3: The selection exploration is written

Check: test -s docs/worker-selection.md

## Verification

Run each check from the repository root and report each exit code. Report
separately which of the three items were delivered.

# TF-016: Delegate tasks to local workers

## Story

As a TaskFactory operator, I want to assign a ready task to a local worker so
the worker can implement and verify it in an isolated checkout while TaskFactory
retains the result and evidence.

## Intent

Local worker delegation is a required capability. The operator can choose a
worker, start one task and retain its commits, verification results, failures
and run metadata. A stalled or failed worker must not silently complete or
archive a task. What remains is stop, resume and hand-off of failed work, and
live progress.

## Delivered

- Worker launch and result contract: TF-023
  (`docs/local-workers-v1.md`).
- OMP, Pi and Claude Code adapters with explicit model, no fallback, local
  endpoint, permission mode and timeout: TF-024, TF-025, TF-026; shared code
  and stall tests: TF-046 (`internal/adapter/common`).
- Manual worker selection, one task per run: TF-056 (`taskfactory work
  <ID> --adapter A --model M [--timeout D]`, no default adapter or model).
- Preflight refusing an unavailable model or endpoint before launch: TF-024,
  TF-025, TF-026 and TF-056 (`TestPreflightRefusalExitsOneAndChangesNothing`).
- Configuration without secrets: flags `--model`, `--haiku-model`,
  `--endpoint`, `--timeout` (`internal/work/args.go`); no key stored
  (`TestNoCredentialReachesLogOrOutput`).
- Stall detection by timeout: TF-046 (`TestLaunchMapsExitStallAndBlocked`).
- A stalled or failed run never completes or archives: TF-056
  (`TestWorkNeverRunsLifecycleCommands`,
  `TestSuggestsFailForBlockedAndStalledWithoutRunningIt`).
- Retained commit, failures and run metadata: worker commit printed by
  `work`, log under `.taskfactory/logs/<ID>/` (TF-055, TF-059, TF-061),
  evidence by `verify` (TF-011), `fail` keeps Claim and evidence (TF-048).
- Split into executable tasks: TF-023, TF-024 to TF-028.
- Real run through a local worker: TF-057
  (`docs/experiments/e2e-local-worker.md`); opt-in smoke test: TF-058.

## Remaining gap

- Stop and resume: TF-028 is still in inbox. The owner deferred the stall
  mapping decision (expected: `fail --outcome BLOCKED`, delivered by TF-048).
  TF-016 stays open until TF-028 is decided and delivered.
- Hand off failed work for retry: needs `requeue` (TF-049, inbox).
- Live progress: `work` writes the log when the run ends; streaming modes
  (`stream-json`, rpc) were deferred in the TF-056 notes.
- Retry budgets: none exist; retries are manual `fail` then `requeue`.
- Evidence that would justify automatic worker selection is not recorded.
  Selection stays manual in v1.

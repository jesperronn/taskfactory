# TF-016: Delegate tasks to local workers

## Story

As a TaskFactory operator, I want to assign a ready task to a local worker so
the worker can implement and verify it in an isolated checkout while TaskFactory
retains the result and evidence.

## Intent

Local worker delegation is a required capability. Explore adapters for OMP, Pi,
and Claude Code through oMLX. The operator should be able to choose a worker,
start one task, observe its progress, and retain its commits, verification
results, failures, and run metadata for later comparison. A stalled or failed
worker must not silently complete or archive a task.

## Refinement needed

- Define the worker launch and result contract, including how TaskFactory passes
  the task and checkout path to each harness.
- Decide how to configure models, local endpoints, permissions, timeouts, and
  retry budgets without storing secrets in task records.
- Define how to detect completion or stalls, stop a run, and resume or hand off
  failed work without losing its Git state or evidence.
- Decide whether worker selection is manual first and what evidence would
  justify automatic selection later.
- Split this story into executable tasks with dependencies, success criteria,
  and verification before promoting any part to `ready`.

TF-023 defines the shared launch and result contract before adapter
implementation tasks are promoted.

## Split into

TF-016 is split into small implementation tasks, all kept in `tasks/inbox/`
until a harness can launch a model and verify them. Each depends on the archived
TF-010 (worktree isolation) and TF-011 (worker evidence); TF-026 also depends on
the TF-015 experiment. Each carries real acceptance checks (`bin/test`,
`go test ./...`, and a documented-invocation comparison against installed CLI
help).

- **TF-024** (inbox) — OMP adapter: launch with explicit `ornith1.5-35B`, no
  fallback, local endpoint, permissions, timeout; runs `bin/test` and
  `bin/lint`. Depends on TF-010, TF-011.
- **TF-025** (inbox) — Pi adapter: same contract via Pi flags; runs `bin/test`
  and `bin/lint`. Depends on TF-010, TF-011.
- **TF-026** (inbox) — Claude Code adapter through the oMLX local endpoint,
  resolving the TF-015 "model not in catalog" warning; no `--fallback-model`.
  Depends on TF-010, TF-011, TF-015.
- **TF-027** (removed) — result commit and evidence recording and the
  independent `verify` call were already delivered by archived TF-011 and
  TF-022, so the proposal was removed as already delivered.
- **TF-028** (inbox) — Worker state machine: completion, failure, stall, stop,
  and resume preserving Git state and evidence. Depends on TF-010, TF-011.

## Context

The first sequential comparison is recorded in
`docs/experiments/TF-015-worker-comparison.md`. It exposed the need to record
incomplete runs as well as commits and to verify worker claims independently.

Planner note: the CLI step that launches these adapters is planned in TF-055 and
TF-056 (see docs/dispatch-design.md).

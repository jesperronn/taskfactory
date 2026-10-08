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

## Context

The first sequential comparison is recorded in
`docs/experiments/TF-015-worker-comparison.md`. It exposed the need to record
incomplete runs as well as commits and to verify worker claims independently.

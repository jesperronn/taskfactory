# TF-023: Define local worker launch and result contract

## Goal

Turn TF-016 into bounded adapter tasks without silently choosing a model or
losing a worker's Git state and verification evidence.

## Dependencies

- TF-010
- TF-011

## Scope

Inspect the installed OMP, Pi, and Claude Code command help and the TF-015
experiment. Write `docs/local-workers-v1.md` with one minimal launch/result
interface: task ID, claimed worktree, explicit worker adapter and model,
project-local endpoint settings, permissions, timeout, observable progress,
exit/blocked/stalled states, and retained commit/evidence paths. Specify a
preflight that refuses an unavailable model or endpoint before launch. The
initial OMP example must explicitly select `ornith1.5-35B` and must not fall
back to Gemma or any default model. Define how the worker runs `bin/test` and
`bin/lint` and how TaskFactory independently calls `verify`. Split TF-016 into
small ready or inbox implementation tasks with dependencies and real acceptance
checks; keep unsupported harness details in inbox until verified.

## Constraints

Do not launch a model, implement an adapter, store secrets in task or evidence
files, or assume all three harnesses expose identical flags. Use observed local
CLI behavior and distinguish it from planned product behavior. Keep manual
worker selection for v1.

## Success criteria

### C1: The launch contract requires explicit adapter and model with no fallback

Check: cat docs/local-workers-v1.md

### C2: Completion, failure, stall, stop, and resume preserve task Git state and evidence

Check: cat docs/local-workers-v1.md

### C3: TF-016 is split into implementation tasks with dependencies and verifiable outcomes

Check: cat tasks/inbox/TF-016-local-worker-delegation.md

### C4: Repository checks and task validation pass

Check: bin/test

## Verification

Manually compare the documented invocations with installed CLI help without
starting a model. Review all new tasks against this contract and the task
validator. Run `bin/test`, `bin/lint`, and whole-tree validation; report exact
exits and any unverified adapter assumptions.

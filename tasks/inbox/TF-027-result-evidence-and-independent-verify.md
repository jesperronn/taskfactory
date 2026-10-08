# TF-027: Record the result commit and call verify independently

## Goal

Retain each run's result commit and evidence, and have TaskFactory call `verify`
independently of the worker's own claim, so a worker's self-reported pass cannot
complete a task.

## Dependencies

- TF-011

## Scope

Record the result commit SHA, changed files, and the evidence path for every run
into `.taskfactory/evidence/<task_id>.jsonl` as an append-only record. Implement
`taskfactory verify <ID>` to run the task's criterion checks in order, then the
configured `verification.worker` commands, and append one evidence record per
attempt — independently of the worker. Stop at the first non-zero exit or shell
start error; do not mark a task complete on a failed or blocked check.

## Constraints

Preserve prior evidence bytes exactly and add no runtime dependency. Record the
empty commit string when a run made no result commit. Keep retries under
orchestrator control; do not build an agent loop.

## Success criteria

### C1: verify runs criterion checks then configured commands and appends one record per attempt

Check: go test ./...

### C2: The result commit and changed files are recorded independently of the worker

Check: go test ./...

### C3: The repository checks pass

Check: bin/test

## Verification

Use a temporary Git project and deterministic task/config commands to verify
execution order, stop behavior, recorded commit and changed files, and PASS,
FAILED, and BLOCKED outcomes. Run `bin/test`, `bin/lint`, and the whole-tree
validator; report exact exits.

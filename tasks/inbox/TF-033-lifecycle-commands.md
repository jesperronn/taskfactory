# TF-033: Task lifecycle commands

## Goal

Close the gap between the protocol roles and the CLI, which only has `status`,
`validate`, `claim`, `verify` and `integrate`.

## Dependencies

- TF-012
- TF-013

## Scope

Specify then implement commands for planner and orchestrator transitions that
today require hand-moving files: promote inbox to ready (only if the ready
contract validates), mark active work failed or blocked with retained evidence,
and requeue a failed task. Each command is atomic, validates the whole task
tree afterwards and commits only the task file moves. Check the spec first and
split into one task per command if large.

## Success criteria

- Each transition is a documented command with refusal tests for invalid states.
- `go run ./cmd/taskfactory validate` passes after every transition.
- No command touches unrelated files.

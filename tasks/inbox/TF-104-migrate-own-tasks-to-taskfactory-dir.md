# TF-104: Move this repository's tasks to .taskfactory/tasks

## Goal

This repository keeps its own task state under `.taskfactory/tasks`, tracked,
with file history preserved, using the layout TF-102 and TF-103 added.

## Dependencies

None

## Scope

This is a data migration of this repository only; it adds no product
behaviour for other projects.

1. `git mv tasks .taskfactory/tasks` in one atomic commit.
2. Update `bin/test`, `bin/lint`, the mdsmith config, `AGENTS.md`,
`docs/worker-instructions.md`, `README.md`, the skill, completion and
`tests/fixtures` to the new paths.
3. Archived tasks keep their old Check lines untouched.
4. Preconditions, checked first: nothing in `tasks/active`, both planning and
worker sessions paused, and TF-102 and TF-103 landed.

## Constraints

Leave in inbox until the preconditions hold; the orchestrator promotes it at a
quiet moment. Not a validator dependency, since dependencies must be archived.
Watch dot-directory blind spots: globbing and lint skip dot paths. Do not edit
archived task bodies. Never touch git signing.

## Success criteria

### C1: The old tasks directory is gone

Check: test ! -e tasks

### C2: The new directory holds the tasks

Check: test -d .taskfactory/tasks/archive

### C3: History is preserved

Check: git log --follow --oneline -- .taskfactory/tasks/archive/TF-078-tf-alias.md

### C4: The whole task tree validates

Check: go run ./cmd/taskfactory validate .taskfactory/tasks

### C5: The full test suite passes

Check: bin/test

## Verification

Run each check, `bin/test` and `bin/lint` from the repository root and report
each exit code.

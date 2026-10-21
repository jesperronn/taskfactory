# TF-104: Move this repository's tasks to .taskfactory/tasks

## Goal

This repository keeps its own task state under `.taskfactory/tasks`, tracked,
with file history preserved, using the layout TF-102 and TF-103 added.

## Dependencies

None

## Scope

This is a data migration of this repository only; it adds no product
behaviour for other projects.

1. `git mv tasks .taskfactory/tasks` in one atomic commit that also sets
`tasks_dir = ".taskfactory/tasks"` in this repository's
`.taskfactory/config.toml`, so no commit exists where config and layout
disagree. The key is the one TF-105 adds.
2. Update `bin/test`, `bin/lint`, the mdsmith config, `AGENTS.md`,
`docs/worker-instructions.md`, `README.md`, the skill, completion and
`tests/fixtures` to the new paths.
3. Archived tasks keep their old Check lines untouched.
4. Preconditions, checked first: TF-102, TF-103 and TF-105 landed, nothing in
`tasks/active`, no claimed worktree left open under `.taskfactory/worktrees`
(TF-093 was stuck active and idle; resolve it first), and both planning and
worker sessions paused.
5. Verify before committing that `bin/lint` (its `find` prune of
`.taskfactory`, `is_task_file`, the `--tells` scope) and the mdsmith config
(its `.taskfactory/**` ignore) still lint Markdown under
`.taskfactory/tasks`; TF-105 changes them, so fix any gap here.

## Constraints

Requires TF-105 landed: do not promote this task before TF-105 is archived.
That order is a constraint, not a validator dependency, because the Dependencies
section must stay `None`. Leave in inbox until the preconditions hold; the
orchestrator promotes it at a quiet moment.
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

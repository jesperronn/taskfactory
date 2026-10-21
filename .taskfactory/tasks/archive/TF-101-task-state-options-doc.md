# TF-101: Write the task state options exploration

## Goal

`docs/task-state-options.md` explains where TaskFactory task state can live,
which option the owner chose, and what a cutover risks, so TF-102 to TF-104
build on a written decision.

## Dependencies

None

## Scope

This task supersedes TF-099, whose exploration it completes. The orchestrator
archives or closes TF-099; do not edit it. Docs only, one new file.

The document covers these sections:

1. Problem: promote, fail and requeue commit task moves, claim and integrate
need a tracked config, and some projects want state outside project history.
2. Options, each with mechanics, effect on every lifecycle command (promote,
claim, verify, fail, requeue, integrate) and cost to the audit trail:
Option A, an external plain directory; Option B, a separate git repository;
Option C, a git-excluded `tasks/` inside the project; Option D, an orphan
branch checked out as a worktree.
3. A comparison of the four options in prose, not a table.
4. Owner decisions, final. First, task state may live outside
`<project>/tasks/` under a separate `stateRoot`; the default layout, state
inside the project at `tasks/` and `.taskfactory/`, stays bit-for-bit as it is
for every other project. Second, history is kept: if the state directory is a
git repository, moves are committed there with the owner's normal git config
and signing is never touched; if it is a plain folder, moves are plain renames.
Third, this repository dogfoods by moving its own `tasks/` to
`.taskfactory/tasks` as the last step, in one atomic commit while no task is
active. Product behaviour for other projects stays separate from this
repository's own data migration.
5. Cutover risks: dot-directory blind spots in globbing and lint, a quiet
moment for the cutover, the hardcoded main branch handled by TF-098, an
external project needing its own worker rules, and the crash window between
the project fast-forward and the state commit, where integration evidence in
`stateRoot` is the source of truth.
6. The follow-up tasks: TF-102 stateRoot, TF-103 commit policy, TF-104 this
repository's migration, in that order.

## Constraints

No code changes. Do not edit TF-099 or any other task. Do not weaken the
current default layout. No tables in task files; the document may use prose and
lists.

## Success criteria

### C1: The document exists

Check: test -s docs/task-state-options.md

### C2: The document lints

Check: bin/lint docs/task-state-options.md

### C3: Option A is covered

Check: grep -q "Option A" docs/task-state-options.md

### C4: Option B is covered

Check: grep -q "Option B" docs/task-state-options.md

### C5: Option C is covered

Check: grep -q "Option C" docs/task-state-options.md

### C6: Option D is covered

Check: grep -q "Option D" docs/task-state-options.md

### C7: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each check, `bin/test` and `bin/lint` from the repository root and report
each exit code.

## Claim

Owner: sonnet-worker-101
Branch: task/TF-101
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-101
Base commit: a7d7130042849432590e2f70a7a962c66600af57
Started at: 2026-10-21T09:30:23Z

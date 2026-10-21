# TF-099: Explore keeping task state out of project history

## Goal

A written exploration says whether and how TaskFactory can run in a project
without committing promote, claim and fail moves to that project's history.

## Dependencies

None

## Scope

- Today `promote`, `fail` and `requeue` commit task moves, and `claim` and
  `integrate` need a tracked config. Some projects want to trial TaskFactory
  without these commits.
- Write `docs/task-state-options.md` comparing options, such as a separate
  state branch or worktree, an untracked `tasks/` directory, or a second
  repository. Cover each lifecycle command and the cost to the audit trail.
- End with a recommendation and, if building is advised, the follow-up task
  written in `tasks/inbox/`.

## Constraints

No code changes. Do not weaken the current default.

## Success criteria

### C1: The exploration document exists

Check: test -s docs/task-state-options.md

### C2: Lint passes

Check: bin/lint docs/task-state-options.md

## Verification

Run both checks from the repository root and report each exit code.

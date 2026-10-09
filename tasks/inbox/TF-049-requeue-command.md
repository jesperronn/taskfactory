# TF-049: Requeue command for failed work

## Goal

Let planners and orchestrators return unclaimed failed work to `tasks/ready/`
or `tasks/inbox/` without hand-moving files. Claimed failed work is never
requeued automatically; it is flagged for a human decision.

## Dependencies

- TF-047
- TF-048

## Scope

Add `taskfactory requeue <ID> [--to ready|inbox]` in a new package
`internal/requeue`, with a thin dispatch in `cmd/taskfactory/main.go`, a
`requeueHelp` constant, and entries in `commandSummaries` and `commandHelps`.
The default target is `ready`.

Behavior:

1. Find exactly one task file for `<ID>` under `tasks/failed/`. Refuse when the
   task is in another state directory or the file is missing.
2. If that file has a `## Claim` block, refuse with exit 1 and no change. The
   message says the task is flagged for a human decision and lists every
   claimed failed task ID found in `tasks/failed/`. Requeue never strips,
   rewrites, or ignores a Claim block.
3. If `.taskfactory/evidence/<ID>.jsonl` exists, refuse with exit 1 and no
   change. An unclaimed task has no evidence, so this only blocks a state that
   needs a human look. Requeue never renames, deletes, or edits evidence. The
   attempt counter restarts at 1 because no evidence file exists.
4. Move the file unchanged to `tasks/ready/` or `tasks/inbox/`. For `ready`,
   the file must satisfy the complete ready contract, or the command refuses
   and leaves it in `tasks/failed/`. For `inbox`, no contract check applies.
5. Run `taskfactory validate tasks`. If it fails, move the file back to
   `tasks/failed/` and exit 1 without committing.
6. Stage only the removed failed path and the added target path. Commit with
   `git commit --no-gpg-sign -m "docs: requeue TF-NNN to <target>"`. Stage
   nothing else. Refuse with exit 1 before any change when the index already
   has staged paths.

Help: `requeue --help` and `requeue -h` print `requeueHelp` and exit 0. The
help states the claimed-work rule. When run inside a project, it also lists
the claimed failed task IDs in `tasks/failed/`, or says none are claimed.

Exit codes: 0 moved and committed; 1 refused or failed; 2 invalid usage
(missing ID, an unknown `--to` value, or an extra argument).

## Constraints

Do not change `claim`, `verify`, `integrate`, `promote`, `fail`, or
`check-main` behavior. Do not touch any file other than the moved task and the
commit paths. Do not archive a task. Add no new Go dependencies; use the
standard library and existing internal packages. The requeue command is
deliberately narrow: claimed failed work is handled by a human decision, not by
this command.

## Success criteria

### C1: An unclaimed failed task with no evidence requeues to ready and commits

Check: go test ./internal/requeue -run Requeues

### C2: A failed task with a Claim block is refused and listed unchanged

Check: go test ./internal/requeue -run RefusesClaimed

### C3: Evidence present or an invalid ready contract is refused

Check: go test ./internal/requeue -run RefusesUnsafe

### C4: --to inbox moves the unclaimed task to inbox and commits

Check: go test ./internal/requeue -run Inbox

### C5: Usage errors exit 2 and requeue --help exits 0

Check: go test ./cmd/taskfactory -run Requeue

### C6: The whole task tree validates after the move

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each C-check, then `bin/test` and `bin/lint` from the repository root. In
a scratch copy, fail an unclaimed task and requeue it to ready, then fail a
claimed task and run requeue on it. Report each exit code, the listed IDs in
the refusal, and `git status` after each run.

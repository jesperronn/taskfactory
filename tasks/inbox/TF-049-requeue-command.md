# TF-049: Requeue command for failed work

## Goal

Let planners and orchestrators return failed work to `tasks/ready/` or
`tasks/inbox/` without hand-moving files, and reset its attempt counter. Claimed
failed work is never requeued automatically; it is flagged for a human decision.

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
3. Reset the attempt counter (owner decision; reading to be confirmed). If
   `.taskfactory/evidence/<ID>.jsonl` exists, its bytes are never edited,
   truncated, or deleted. Requeue renames the file aside to
   `.taskfactory/evidence/<ID>.attempts-<N>.jsonl`, where `N` is the highest
   `attempt` value in the old file. The rename never overwrites: if the aside
   path already exists, refuse with exit 1 and no change. The next attempt then
   starts at 1. The old history stays whole under the aside name, so the
   protocol rule against rewriting existing evidence still holds. The command
   output and the commit message body state that the counter was reset and
   name the aside path. If the requeue fails after the rename, the rename is
   undone.
4. Move the file unchanged to `tasks/ready/` or `tasks/inbox/`. For `ready`,
   the file must satisfy the complete ready contract, or the command refuses
   and leaves it in `tasks/failed/`. For `inbox`, no contract check applies.
5. Run `taskfactory validate tasks`. If it fails, move the file back to
   `tasks/failed/`, rename any aside evidence file back, and exit 1 without
   committing.
6. Stage only the removed failed path and the added target path. Commit with
   `git commit -m "docs: requeue TF-NNN to <target>"` as plain Git, so the
   user's signing configuration applies. Never pass `--no-gpg-sign`, `-c
   commit.gpgsign=false`, or any other signing override. When a reset happened,
   the body names the aside path. The aside evidence file is untracked and is
   never staged. Stage nothing else. Refuse with exit 1 before any change when
   the index already has staged paths.

Help: `requeue --help` and `requeue -h` print `requeueHelp` and exit 0. The
help states the claimed-work rule and the counter reset. Inside a project it
also lists the claimed failed task IDs in `tasks/failed/`, or says none are
claimed. Outside a project it prints the static text only.

Exit codes: 0 moved and committed; 1 refused or failed; 2 invalid usage
(missing ID, an unknown `--to` value, or an extra argument).

## Constraints

Do not change `claim`, `verify`, `integrate`, `promote`, `fail`, or
`check-main` behavior. Do not touch any file other than the moved task, the
aside evidence rename, and the commit paths. Do not archive a task. Add no new
Go dependencies; use the standard library and existing internal packages.
Evidence bytes are never edited, truncated, or deleted; only the rename in
step 3 moves them. Test fixtures that create a temporary repository set
`commit.gpgsign` to `false` in that repository's own configuration only.

## Success criteria

### C1: An unclaimed failed task requeues to ready and commits

Check: go test ./internal/requeue -run Requeues

### C2: A failed task with a Claim block is refused and listed unchanged

Check: go test ./internal/requeue -run RefusesClaimed

### C3: An invalid ready contract or an existing aside path is refused

Check: go test ./internal/requeue -run RefusesUnsafe

### C4: Evidence is renamed aside with the counter reset and bytes kept

Check: go test ./internal/requeue -run ResetsAttempts

### C5: --to inbox moves the unclaimed task to inbox and commits

Check: go test ./internal/requeue -run Inbox

### C6: Usage errors exit 2 and requeue --help exits 0

Check: go test ./cmd/taskfactory -run Requeue

### C7: No non-test Go file in internal/requeue overrides commit signing

Check: go test ./internal/requeue -run CheckNoSigningOverride

### C8: The whole task tree validates after the move

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each C-check, then `bin/test` and `bin/lint` from the repository root. In
a scratch copy, record a FAILED attempt for an active task with `verify`, run
`taskfactory fail <ID> --outcome FAILED`, then `taskfactory requeue <ID>`.
Report the exit code, the aside file name, `cmp` of the aside file against the
original evidence bytes, and `git status` after each run. Run `requeue` on a
claimed failed task and report its exit code and the listed IDs.

# TF-047: Promote command for inbox to ready

## Goal

Give the planner a single command that moves an inbox proposal to
`tasks/ready/` only when its complete executable contract validates, so
promotion no longer requires hand-moving files.

## Dependencies

- TF-003

## Scope

Add `taskfactory promote <ID>` in a new internal package (for example
`internal/promote`) with a thin dispatch in `cmd/taskfactory/main.go`.

Behavior:

1. Find exactly one task file for `<ID>` under `tasks/inbox/`. Refuse when it
   is in any other state directory or does not exist.
2. Validate the file with the ready contract (the same rules as
   `taskfactory validate tasks/inbox/<file>` would apply after a move to
   `tasks/ready/`). Refuse with a diagnostic and no change when it fails.
3. Move the file with `git mv` semantics or an atomic rename to
   `tasks/ready/<same file name>`. Do not edit the file contents.
4. Run `taskfactory validate tasks` over the whole tree. If it fails, restore
   the file to `tasks/inbox/` and exit 1 without committing.
5. Stage only the two task paths (the removed inbox path and the new ready
   path) and commit them with a message such as `docs: promote TF-NNN to ready`
   using `git commit --no-gpg-sign`. Stage nothing else.

Exit codes: 0 promoted and committed; 1 refused or failed (including validation
or commit failure, with the file restored); 2 invalid usage.

Help text follows the style of `claimHelp`, `verifyHelp` and the aligned
command list in `usage`.

## Constraints

Do not change the `claim`, `verify`, `integrate` or `validate` behavior. Do not
touch any file other than the promoted task. No new Go dependencies; use only
the standard library and existing internal packages. Never write a `## Claim`
block on promotion. Do not stage or commit unrelated working-tree changes; if
the index already contains staged paths, refuse with exit 1.

## Success criteria

### C1: Valid inbox task is promoted and committed

Check: go test ./internal/promote -run Promotes

### C2: Invalid contract, wrong state, or unknown ID is refused unchanged

Check: go test ./internal/promote -run Refuses

### C3: Unrelated staged or dirty files are never committed

Check: go test ./internal/promote -run Isolation

### C4: Help and exit codes match the other commands

Check: go test ./cmd/taskfactory -run Promote

### C5: Whole task tree validates after promotion

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run the five checks, then `bin/test` and `bin/lint`. In a scratch copy of the
repository, promote one valid inbox task and one invalid inbox task; report the
exit code and `git status` for each. Confirm `promote --help` exits 0 and
`promote` with no argument exits 2.

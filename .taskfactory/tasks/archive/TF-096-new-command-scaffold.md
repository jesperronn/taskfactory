# TF-096: Scaffold inbox tasks with taskfactory new

## Goal

`taskfactory new "<title>"` writes a ready-to-edit task skeleton in
`tasks/inbox/`, so a planner starts from a correct contract instead of a blank
file.

## Dependencies

None

## Scope

- Add the `new` command. It picks the next free `TF-NNN` across every state
  directory including the archive, derives a slug from the title (lowercase
  letters, digits, single hyphens), and writes `tasks/inbox/<ID>-<slug>.md`.
- The skeleton has every ready heading in order, one `### C1:` criterion with
  a `Check:` line, and `TODO` markers where prose is needed.
- Support `--dry-run` and `--verbose` like the other commands, and print a
  `next:` hint pointing at `validate` and `promote`.
- Refuse an empty title or a title with no usable slug characters.
- It writes only the one file and commits nothing.

## Constraints

Do not change the task format or the validator. The skeleton must pass
`taskfactory validate` as an inbox file. `promote` must refuse it while
`TODO` markers remain.

## Success criteria

### C1: New writes a valid skeleton with the next free ID

Check: go test ./cmd/taskfactory -run TestNew -count=1

### C2: Promote refuses a skeleton that still has TODO markers

Check: go test ./internal/promote -run TestPromoteRefusesTodo -count=1

### C3: The whole suite and lint still pass

Check: bin/test && bin/lint

## Verification

Run the three checks from the repository root and report each exit code.

## Claim

Owner: sonnet-worker-096
Branch: task/TF-096
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-096
Base commit: 2954270410742b2dcf763978b7c15e2bf6ae41a8
Started at: 2026-10-21T20:31:47Z

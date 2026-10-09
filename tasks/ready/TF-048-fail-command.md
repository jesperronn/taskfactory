# TF-048: Fail command for active work

## Goal

Give the orchestrator a command that moves a claimed task from
`tasks/active/` to `tasks/failed/` with its Claim block and attempt evidence
retained. A worker that stalls or is stopped is recorded the same way, with
outcome `BLOCKED`, so no hand-moved file is needed.

## Dependencies

- TF-009
- TF-011

## Scope

Add `taskfactory fail <ID> --outcome <FAILED|BLOCKED> [--reason <text>]` in a
new package `internal/fail`, with a thin dispatch in `cmd/taskfactory/main.go`,
a `failHelp` constant, and entries in `commandSummaries` and `commandHelps`.

Behavior:

1. Find exactly one task file for `<ID>` under `tasks/active/`. Refuse when the
   task is in another state directory or the file is missing.
2. Require a complete, valid `## Claim` block. Refuse otherwise. Do not
   fabricate metadata.
3. Inspect `.taskfactory/evidence/<ID>.jsonl`:
   - When the file has records, the last record's `outcome` must equal
     `--outcome`. Refuse otherwise and name both values. `--reason` is refused
     with exit 1 when evidence exists.
   - When the file is missing or empty, refuse unless `--reason <text>` is
     given. The reason must be non-empty single-line text.
4. Move the file to `tasks/failed/` unchanged. The Claim block and every byte
   are retained.
5. Run `taskfactory validate tasks`. If it fails, move the file back to
   `tasks/active/` and exit 1 without committing.
6. Stage only the removed active path and the added failed path. Commit with
   `git commit -m "docs: mark TF-NNN failed"` as plain Git, so the user's
   signing configuration applies. Never pass `--no-gpg-sign`, `-c
   commit.gpgsign=false`, or any other signing override. When `--reason` was
   used, its text is added as the commit message body, because a verify
   evidence record must hold at least one check result and cannot carry a
   reason. Never create, modify, or stage the evidence file.

`BLOCKED` is an evidence outcome, so it is recorded in evidence and not as a
separate file state. A stalled or stopped worker is recorded by running the
command with `--outcome BLOCKED` after a BLOCKED evidence record exists. A
BLOCKED task moves to `tasks/failed/` exactly as a FAILED task does. This
command never archives or completes a task.

Exit codes: 0 moved and committed; 1 refused or failed; 2 invalid usage
(missing `--outcome`, an outcome other than `FAILED` or `BLOCKED`, an empty
`--reason`, or an unknown or extra argument).

`fail --help` and `fail -h` print `failHelp` and exit 0, in the style of
`claimHelp`, with a Flags section and an Exit codes section.

## Constraints

Do not change `claim`, `verify`, `integrate`, `promote`, or `check-main`
behavior. Do not touch any file other than the moved task and the commit
paths. Do not archive a task; archive still requires integration. Add no new
Go dependencies; use the standard library and existing internal packages. If
the index already has staged paths, refuse with exit 1 before any change.
Test fixtures that create a temporary repository set `commit.gpgsign` to
`false` in that repository's own configuration only, so they do not depend on
the developer's global Git configuration.

## Success criteria

### C1: An active claimed task with matching evidence moves to failed

Check: go test ./internal/fail -run Moves

### C2: Missing Claim, mismatched outcome, or missing evidence is refused

Check: go test ./internal/fail -run Refuses

### C3: Evidence bytes and the Claim block are unchanged after the move

Check: go test ./internal/fail -run Retains

### C4: --reason is accepted only without evidence and is committed

Check: go test ./internal/fail -run Reason

### C5: Usage errors exit 2 and fail --help exits 0

Check: go test ./cmd/taskfactory -run Fail

### C6: No non-test Go file in internal/fail overrides commit signing

Check: go test ./internal/fail -run CheckNoSigningOverride

### C7: The whole task tree validates after the move

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each C-check, then `bin/test` and `bin/lint` from the repository root. In
a scratch copy, claim a task, record a FAILED attempt with `verify`, run
`taskfactory fail <ID> --outcome FAILED`, and report the exit code,
`git status`, and `diff` of the evidence file before and after. Repeat with a
BLOCKED start error and `--outcome BLOCKED`. Report each exit code.

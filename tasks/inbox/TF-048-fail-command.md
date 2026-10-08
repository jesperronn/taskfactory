# TF-048: Fail command for active work

## Goal

Give the orchestrator a command that moves a claimed task from
`tasks/active/` to `tasks/failed/` with its Claim block and attempt evidence
retained, so failed or blocked work no longer needs a hand-moved file.

## Dependencies

- TF-009
- TF-011

## Scope

Add `taskfactory fail <ID> --outcome <FAILED|BLOCKED>` in a new internal
package (for example `internal/fail`) with a thin dispatch in
`cmd/taskfactory/main.go`.

Behavior:

1. Find exactly one task file for `<ID>` under `tasks/active/`. Refuse when the
   task is in another state or the file is missing.
2. Require the active file to have a complete, valid `## Claim` block. Refuse
   otherwise; do not fabricate metadata.
3. Require that `.taskfactory/evidence/<ID>.jsonl` exists and that its last
   record's `outcome` equals the `--outcome` value. See the open question in
   Notes.
4. Move the file to `tasks/failed/` unchanged. The Claim block and all bytes
   are retained.
5. Run `taskfactory validate tasks`. If it fails, move the file back to
   `tasks/active/` and exit 1 without committing.
6. Stage only the removed active path and the added failed path, and commit
   with `git commit --no-gpg-sign` using a message such as
   `docs: mark TF-NNN failed`. Stage nothing else. Never modify or append the
   evidence file.

Exit codes: 0 moved and committed; 1 refused or failed; 2 invalid usage
(missing `--outcome`, an outcome other than `FAILED` or `BLOCKED`, or an extra
argument).

Help text follows the style of `claimHelp` and the aligned command list in
`usage`.

## Constraints

Do not change `claim`, `verify` or `integrate` behavior. Do not touch any file
other than the moved task and the commit paths. Do not archive a task; archive
still requires integration. No new Go dependencies. Do not stage or commit
unrelated changes; refuse with exit 1 when the index already has staged paths.

## Success criteria

### C1: Active claimed task with matching evidence moves to failed

Check: go test ./internal/fail -run Moves

### C2: Missing Claim, missing evidence, or mismatched outcome is refused

Check: go test ./internal/fail -run Refuses

### C3: Evidence bytes and the Claim block are unchanged

Check: go test ./internal/fail -run Retains

### C4: Usage errors exit 2 and help is printed

Check: go test ./cmd/taskfactory -run Fail

### C5: Whole task tree validates after the move

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run the five checks, then `bin/test` and `bin/lint`. In a scratch copy, claim a
task, record a FAILED attempt, run `fail <ID> --outcome FAILED`, and report the
exit code, `git status`, and `diff` of the evidence file before and after.

## Notes

- Open question: must `--outcome` match the last evidence record, or may the
  orchestrator record a failure that has no evidence (for example an
  environment problem before any verify run)? This proposal refuses in that
  case. The protocol says failure details belong in attempt evidence, so the
  planner should confirm.
- Open question: `docs/TASKFACTORY-SPEC.md` has no separate blocked state, and
  `tasks/failed/` holds "failed or blocked" work. Whether `BLOCKED` is a
  distinct recorded outcome in the failed file or only in evidence is not
  stated beyond the protocol's outcome list.

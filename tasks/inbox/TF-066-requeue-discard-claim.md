# TF-066: Requeue can discard a claim

## Goal

Let an owner decide a claimed failed task with one command instead of editing
the task file by hand. `taskfactory requeue` today refuses every failed task
that still has a Claim block; the only way forward is a manual edit.

## Dependencies

- TF-048
- TF-049

## Scope

Add `--discard-claim` to `taskfactory requeue <ID> [--to ready|inbox]` in
`internal/requeue`, with the flag parsed, helped and dispatched in
`cmd/taskfactory/main.go`. Owner decision, final:

1. Without the flag, a failed task that still has a Claim block is refused
   exactly as today (exit 1, flagged for a human decision, nothing changes).
   The message now also names `--discard-claim` as the way to decide.
2. With the flag, requeue removes the whole `## Claim` block from the task
   file, then proceeds as for an unclaimed task. The evidence file is renamed
   aside to `<ID>.attempts-<N>.jsonl` with the counter reset. A ready target
   is accepted only if the file then validates as a complete ready contract.
   Tracked and untracked failed files are handled as `internal/fail` handles
   them, and only the two task paths are staged.
3. The commit is a plain `git commit` that respects the user's signing
   configuration. No signing override appears anywhere in product code. The
   commit message body records the discarded claim: owner, branch, base commit
   and worktree path, exactly as the block had them.
4. Any failure restores the file bytes, its location and the evidence name.
5. `--discard-claim` on a task without a Claim block is accepted, discards
   nothing and says so. It is not an error.
6. Requeue never deletes the worktree or the branch of the discarded claim.
   The README documents the two git commands that remove them by hand.
7. `--help` documents the flag, dynamic inside a project and static outside.
   Exit codes stay 0, 1 and 2. A value given to the flag, a repeated flag and
   an unknown flag exit 2.

Update the README requeue paragraph, the skill command table in
`skills/taskfactory/SKILL.md` and the Requeue section of
`docs/protocol-v1.md`.

## Constraints

Do not change `claim`, `verify`, `integrate`, `promote`, `fail` or
`check-main` behavior. Without the flag, requeue behavior is unchanged. Add no
Go dependencies. Evidence bytes are never edited, truncated or deleted. Test
fixtures that create a temporary repository set `commit.gpgsign` to `false` in
that repository's own configuration only. Do not archive this task.

## Success criteria

### C1: A claimed task requeues to ready with the claim removed

Check: go test ./internal/requeue -run DiscardClaimToReady -v

### C2: A claimed task requeues to inbox with the claim removed

Check: go test ./internal/requeue -run DiscardClaimToInbox -v

### C3: The commit body records owner, branch, base commit and worktree

Check: go test ./internal/requeue -run DiscardClaimBody -v

### C4: Without the flag a claimed task is refused and the flag is named

Check: go test ./internal/requeue -run DiscardClaimNeeded -v

### C5: Evidence is renamed aside and the rest of the file is unchanged

Check: go test ./internal/requeue -run DiscardClaimKeepsRest -v

### C6: An unclaimed task with the flag is a no-op note, not an error

Check: go test ./internal/requeue -run DiscardClaimUnclaimed -v

### C7: Failures restore the file, the claim and the evidence name

Check: go test ./internal/requeue -run DiscardClaimRestores -v

### C8: Flag parsing, usage errors and help exit with the right codes

Check: go test ./cmd/taskfactory -run RequeueDiscardClaim -v

### C9: No non-test Go file in internal/requeue overrides commit signing

Check: go test ./internal/requeue -run CheckNoSigningOverride -v

### C10: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each C-check, then `bin/test` and `bin/lint` from the repository root. In
a scratch repository claim a task, make it fail with `taskfactory fail`, and
report the exit code of `taskfactory requeue` without the flag (1), then with
`--discard-claim --to ready` (0). Report the evidence listing, the committed
task paths, `git status --short` and the requeue commit message.

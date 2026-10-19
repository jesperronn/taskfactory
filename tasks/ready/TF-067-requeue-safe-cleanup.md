# TF-067: Requeue safely cleans the old worktree and branch

## Goal

Let a requeued task be claimed again. After `requeue`, a re-claim fails with
`branch task/<ID> already exists` because the old worktree and branch remain.
`requeue` now removes them when that is safe, and otherwise refuses.

## Dependencies

- TF-049
- TF-066

## Scope

Change `internal/requeue`, with help text in `cmd/taskfactory/main.go`.
Owner decision, final:

1. Which worktree and branch. When the task has a Claim block, the ones
   recorded in it (Worktree and Branch), read before the block is removed;
   that is the `--discard-claim` path. With no Claim block, the conventional
   `.taskfactory/worktrees/<ID>` and `task/<ID>`, when they exist. A worktree
   path outside the project's `.taskfactory/worktrees` is never touched:
   requeue refuses.
2. Cleanup is safe only when all hold. The worktree directory is clean:
   `git -C <worktree> status --porcelain` prints nothing, untracked files
   included. The branch has no commits outside local main:
   `git rev-list main..<branch>` is empty, so its tip is an ancestor of main;
   a branch still at its Claim base commit qualifies. The worktree is not the
   project's own main checkout.
3. The safety checks run first, before any change. If one is unmet, requeue
   exits 1 and changes nothing (task, evidence, files). It says which
   condition failed: dirty files, or N unmerged commits with
   `git log --oneline main..<branch>` as the way to inspect them. It prints the
   manual commands: `git worktree remove <path>` then `git branch -d <branch>`,
   or `-D` if the human decides to discard the commits.
4. If safe, the cleanup runs last, after the requeue commit succeeded:
   `git worktree remove <path>` without `--force`, then
   `git branch -d <branch>`, never `-D`. If the cleanup fails after the commit,
   requeue exits 1 and states plainly that the task was requeued and committed,
   and what was left behind. The success output reports what was removed. A worktree or
   branch that is already missing is not an error.
5. Nothing is removed when requeue is refused for any other reason.
6. Unmerged worker commits always need the human to decide. Failed tasks
   usually carry worker commits, so the refusal is the common case and the
   manual commands remain the way. The docs say so honestly.

Update the README requeue paragraph (replace the manual removal before a
re-claim, keep the commands for the refusal case), the skill command table in
`skills/taskfactory/SKILL.md`, `requeueHelp` and the Requeue section of
`docs/protocol-v1.md`.

## Constraints

Do not change `claim`, `verify`, `integrate`, `promote`, `fail` or
`check-main` behavior. Cleanup never uses `--force` or `-D`. Add no Go
dependencies. Evidence bytes are never edited, truncated or deleted. No signing
override appears anywhere in product code. Test fixtures that create a
temporary repository set `commit.gpgsign` to `false` in that repository's own
configuration only. Do not archive this task.

## Success criteria

### C1: A safe cleanup removes both and a re-claim succeeds

Check: go test ./internal/requeue -run CleanupSafeRemovesAndReclaims -v

### C2: A dirty worktree refuses and changes nothing

Check: go test ./internal/requeue -run CleanupDirtyRefuses -v

### C3: Unmerged commits refuse with the count and the commands

Check: go test ./internal/requeue -run CleanupUnmergedRefuses -v

### C4: A branch merged into main is cleaned

Check: go test ./internal/requeue -run CleanupMergedBranch -v

### C5: A branch still at the base commit is cleaned

Check: go test ./internal/requeue -run CleanupBaseCommitBranch -v

### C6: A worktree outside the worktree root is refused

Check: go test ./internal/requeue -run CleanupOutsideRoot -v

### C7: A missing worktree or branch is not an error

Check: go test ./internal/requeue -run CleanupMissing -v

### C8: A cleanup failure after the commit is reported honestly

Check: go test ./internal/requeue -run CleanupFailureAfterCommit -v

### C9: No non-test Go file in internal/requeue overrides commit signing

Check: go test ./internal/requeue -run CheckNoSigningOverride -v

### C10: Help describes the cleanup and the refusal commands

Check: go test ./cmd/taskfactory -run RequeueCleanupHelp -v

### C11: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each C-check, then `bin/test` and `bin/lint` from the repository root. In
a scratch repository claim a task, fail its verify, run `taskfactory fail`,
then `taskfactory requeue --discard-claim --to ready` with a clean worktree
and no worker commits: report the exit code (0), `git worktree list`,
`git branch`, and that a second claim succeeds. In a second scenario the
worker committed a change not on main: report that requeue exits 1 and that
`git status` and `git ls-tree -r HEAD` show nothing changed.

# TF-012: Integrate one verified candidate

## Goal

Serialize candidate integration onto current main with rebase, verification, and
ff-only merge.

## Dependencies

TF-010 and TF-011 must be archived before promotion to ready.

## Scope

Implement `taskfactory integrate <ID>` for one passing active task. Acquire a
local integration lock, check the current local `main` ref, rebase the candidate
onto that commit, run integration verification, and advance local main with
`git merge --ff-only`. Remote fetch/push is outside this task. Keep the lock
through verification and merge. If local main moved before merge, rebase and
verify the new candidate commit again. Archive after the merge and required
candidate checks. If post-merge main verification is configured, run it while
holding the lock and do not archive on failure; TF-013 adds persistent
stopped-state and recovery. Never claim the merge was rolled back after a
post-merge failure. Record failures without silently changing feature code.

## Success criteria

- A verified candidate advances main without a merge commit and moves to archive
  when all configured checks pass.
- A stale candidate is rebased and reverified before merge; recorded evidence
  identifies the exact verified and merged commit.
- Rebase, verification, and ff-only failures leave the task unarchived with
  evidence.
- Simultaneous integrations cannot both enter the critical section.
- `go test ./...` and `go vet ./...` pass.

## Verification

Use temporary local Git repositories for success, stale-main, and failure cases;
inspect commit ancestry, task state, and evidence.

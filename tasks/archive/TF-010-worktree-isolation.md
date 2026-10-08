# TF-010: Claim a task into an isolated worktree

## Goal

Expose `taskfactory claim <ID> --owner <name>` and complete the protocol-level
ready-to-active transition with a dedicated Git branch and worktree.

## Dependencies

- TF-009

## Scope

Use `internal/claim.WithClaimLock` around eligibility, worktree setup, and the
task state transition. Create branch `task/<ID>` from the current full main
commit and a worktree at `<git.worktree_root>/<ID>` with direct Git
subprocesses. Record complete Claim metadata as specified in
`docs/task-format-v1.md`: owner, branch, canonical worktree path, base commit,
and UTC start time. Require a non-empty `--owner` value; do not infer an agent
identity. Move the task from ready to active only after the branch and worktree
exist, and make the complete active file visible before reporting success.
Validate the resulting active contract. On a failed Git or file operation, undo
only resources created by this attempt and restore the ready task. If cleanup
itself fails, report the exact remaining branch, path, and task state for manual
recovery.

## Constraints

Do not run a worker or integrate its result. Do not overwrite an existing
branch, path, or task file. Preserve task contract bytes apart from the appended
Claim block. Use the existing Go standard library, config loader, validator, and
claim guard; do not add a dependency.

## Success criteria

### C1: A successful claim creates matching branch, worktree, and active metadata

Check: go test ./...

### C2: Concurrent claims of one task yield exactly one success and respect capacity

Check: go test ./...

### C3: Branch and path conflicts leave the task ready and preserve existing resources

Check: go test ./...

### C4: A failed setup restores ready or reports an explicit recoverable failure

Check: go test ./...

### C5: Repository checks pass

Check: bin/test

## Verification

Use temporary Git repositories and inspect `git worktree list --porcelain`,
branch refs, task paths, and Claim metadata. Test two distinct task IDs, same-ID
concurrency, configured capacity, a preexisting branch, an occupied worktree
path, and an injected setup failure. Confirm no unowned cleanup and that
whole-tree validation passes after success and rollback. Run `bin/test`,
`bin/lint`, `go vet ./...`, and the repository validator; report exact exits.

# TF-010: Give a claimed worker an isolated worktree

## Goal

Create a branch and Git worktree for a claimed task without sharing a mutable
checkout.

## Dependencies

TF-009 must be archived before promotion to ready.

## Scope

Expose `taskfactory claim <ID>` and extend the internal claim to create a task
branch/worktree from the recorded base commit and store branch, path, and base
commit in active metadata. Use direct Git subprocesses. Use branch `task/<ID>`
and a per-task worktree path defined by `docs/config-v1.md`. If branch or
worktree setup fails, undo only resources created by this attempt and return the
task to ready; if cleanup fails, retain an explicit failed claim record and
require manual recovery. Never report success without an existing branch,
worktree, and matching active metadata. Do not implement worker execution or
integration.

## Success criteria

- Two claims create distinct worktrees and task branches from their recorded
  bases.
- An existing conflicting branch/path produces a useful error without
  overwriting files.
- A failed Git operation either restores the ready task or records an explicit
  recoverable failure; it never leaves a falsely successful active task.
- `go test ./...` and `go vet ./...` pass.

## Verification

Test with temporary local Git repositories and inspect
`git worktree list --porcelain`, branch refs, and task metadata.

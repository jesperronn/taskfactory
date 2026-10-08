# TF-009: Claim a ready task exactly once

## Goal

Implement atomic ready-to-active claiming with dependency and capacity checks.

## Dependencies

TF-002 and TF-003 must be archived before promotion to ready.

## Scope

Add an internal claim operation for a local project. Do not expose the CLI
command until TF-010 adds worktree creation. Resolve dependencies against
archive, count active workers against configured maximum, and atomically move
one ready task to active under one local claim lock covering the
dependency/capacity checks and move. A filesystem rename alone does not protect
the worker-capacity check. Record owner and start metadata as defined by
`docs/task-format-v1.md`; a failure before the move leaves the ready file
unchanged. This task does not create a Git branch/worktree; keep the
intermediate operation internal until TF-010 completes the protocol-level claim.

## Success criteria

- Two simultaneous claim attempts for one ID produce exactly one success.
- Missing/unarchived dependency and full capacity reject the claim without
  moving the task; simultaneous claims of distinct tasks cannot exceed
  configured capacity.
- Successful claim retains the original contract and records required owner
  metadata.
- `go test ./...` and `go vet ./...` pass.

## Verification

Use temporary task trees and a concurrent claim test. Inspect the resulting
active task and confirm no duplicate or partial task file remains.

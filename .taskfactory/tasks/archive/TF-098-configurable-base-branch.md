# TF-098: Support a base branch other than main

## Goal

Projects whose default branch is `master` (or any other name) can use claim,
integrate, check-main and requeue.

## Dependencies

None

## Scope

- Today claim, integrate, check-main and requeue cleanup hardcode
  `refs/heads/main`. Add an optional `git.base_branch` config key that
  defaults to `main`, and use it in all four places.
- Update `docs/config-v1.md` and the README text that says main.
- Keep error messages naming the configured branch.

## Constraints

An existing config without the key must behave exactly as before. Do not
auto-detect the branch.

## Success criteria

### C1: A repo on master completes claim to integrate

Check: go test ./internal/integrate -run TestBaseBranch -count=1

### C2: Claim and requeue honour the configured branch

Check: go test ./internal/claim ./internal/requeue -run BaseBranch -count=1

### C3: The whole suite and lint still pass

Check: bin/test && bin/lint

## Verification

Run the three checks from the repository root and report each exit code.

## Claim

Owner: sonnet-worker-098
Branch: task/TF-098
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-098
Base commit: 1f481dd0c925bb04d239be8ea903a5a0c09168b5
Started at: 2026-10-21T21:26:52Z

# TF-114: Promote creates a missing ready directory

## Goal

`taskfactory promote` fails with "no such file or directory" when
`.taskfactory/tasks/ready` does not exist. Git does not track empty
directories, and the directory vanishes when its last task moves out, so a
fresh clone or a cleaned tree breaks promote. Make promote create the target
state directory, and make `claim` and `requeue` do the same for theirs.

## Dependencies

None

## Scope

- Find where tasks move between state directories (promote, claim, requeue,
  fail, integrate) and create the destination directory before the rename.
- Add a test per command that starts with the destination directory removed
  and shows the move succeeds.
- Keep behaviour unchanged when the directory exists.

## Constraints

No change to the task contract, the config keys or the commit policy. Do not
touch commit signing.

## Success criteria

### C1: Promote works without a ready directory

Check: go test ./internal/promote -run TestPromoteCreatesMissingReadyDir

### C2: The full test suite passes

Check: bin/test

### C3: Lint passes

Check: bin/lint

## Verification

Run each check from the repository root and report each exit code.

## Claim

Owner: haiku-worker-114
Branch: task/TF-114
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-114
Base commit: a9243b4a5c938b60a28d9488d0650ed3b1ca3fb3
Started at: 2026-10-23T04:23:37Z

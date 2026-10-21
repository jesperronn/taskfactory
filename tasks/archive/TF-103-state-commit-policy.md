# TF-103: Commit policy for an external state directory

## Goal

When `stateRoot` is a git repository, task moves are committed there; when it
is a plain folder they are plain renames, and a crash between the project
fast-forward and the state commit is recoverable.

## Dependencies

None

## Scope

Product behaviour only; this repository's own data does not move here.

1. If `stateRoot` is a git repository, promote, fail, requeue and the
integrate archive step commit there (`git -C stateRoot`) with the same
messages as today, using the owner's git config; signing is never overridden.
2. The guards that require a task tracked in HEAD (fail, integrate) check the
state repository instead of the project. A plain folder skips those guards and
uses renames and file-based restore on failure.
3. Crash window: if the project fast-forward succeeded but the state commit did
not, the integration evidence in `stateRoot` is the source of truth. The next
integrate run reads it, finishes the archive and commit, and never merges
twice.
4. `taskfactory init --state-dir [path]` writes the state layout, the config
with `project_root`, and documents it in help and the README.
5. The default layout (state inside the project) behaves as today.

## Constraints

Order: land after TF-102 (not a validator dependency, since dependencies must
be archived) and before TF-104. Never override commit signing or identity.

## Success criteria

### C1: Moves commit in a git state repository

Check: go test ./internal/promote -run TestStateRepoCommit -count=1

### C2: A plain state folder uses renames

Check: go test ./internal/fail -run TestPlainStateRename -count=1

### C3: Tracked-in-HEAD guards use the state repository

Check: go test ./internal/integrate -run TestStateRepoTrackedGuard -count=1

### C4: A crash between fast-forward and state commit recovers

Check: go test ./internal/integrate -run TestStateCommitCrashWindow -count=1

### C5: Init with a state dir writes the layout

Check: go test ./cmd/taskfactory -run TestInitStateDir -count=1

### C6: The full test suite passes

Check: bin/test

### C7: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each check, `bin/test` and `bin/lint` from the repository root and report
each exit code.

## Claim

Owner: sonnet-worker-103
Branch: task/TF-103
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-103
Base commit: 5bdcc695879ca0345cf1864fc87fb7e2b5b7aa45
Started at: 2026-10-21T09:59:53Z

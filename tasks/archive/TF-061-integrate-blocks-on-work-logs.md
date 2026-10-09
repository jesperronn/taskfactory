# TF-061: Let integrate ignore the work log directory

## Goal

`taskfactory integrate` must succeed after `taskfactory work` has written its
log, without the operator moving the log by hand.

## Dependencies

- TF-012
- TF-056

## Scope

Found in the TF-057 end-to-end run: integrate refused with "unrelated main
worktree change blocks integration: .taskfactory/logs/TF-001/pi-...log" and,
after adding the directory to `.git/info/exclude`, with "unexpected runtime
path .taskfactory/logs". Only the claim lock, the integration lock and the
evidence paths were allowed in `internal/integrate/integrate.go`.

Owner decision: `.taskfactory/logs/` is an allowed runtime path for
`integrate`, like the claim lock, the integration lock and the evidence paths.
It is written by `taskfactory work` and is never tracked. Its presence, even
with files in it, must not block integration, must not be staged or committed
and must not cause a FAILED integration evidence record. Both the status check
and the runtime inventory in `internal/integrate` allow it.

Nothing else becomes allowed. A tracked or untracked change anywhere else still
blocks. A path that only starts with the same prefix, such as
`.taskfactory/logs-old` or `.taskfactory/logsx`, is still unexpected.

Update every place that lists the allowed runtime paths so docs and code agree:
`docs/protocol-v1.md`, `docs/technical-spec-v1.md`,
`docs/worker-instructions.md`, `README.md` and `skills/taskfactory/SKILL.md`.

## Constraints

Do not change existing integrate tests. Do not stage, commit or delete the log
directory. Never override Git signing in product code; test fixtures disable
signing in their own repository config only. No new Go dependencies.

## Success criteria

### C1: A log directory with files does not block integration

Check: go test ./internal/integrate -run LogsDirAllowed -v

### C2: The logs stay untracked and out of the integration commit

Check: go test ./internal/integrate -run LogsStayUntracked -v

### C3: Lookalike paths and unrelated files still block

Check: go test ./internal/integrate -run LogsLookalikeBlocks -v

### C4: A successful run records no failure evidence

Check: go test ./internal/integrate -run LogsNoFailedEvidence -v

### C5: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each C-check, then `bin/test` and `bin/lint` and report each exit code. In
a scratch repository built with the real binary, run init, promote, claim, a
worker commit, `work` with a fake adapter on PATH, verify and integrate without
moving the log. Report the outputs, `git status --short` and
`git show --stat HEAD`.

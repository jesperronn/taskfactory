# TF-061: Let integrate ignore the work log directory

## Goal

`taskfactory integrate` must succeed after `taskfactory work` has written its
log, without the operator moving the log by hand.

## Scope

Found in the TF-057 end-to-end run: integrate refused with "unrelated main
worktree change blocks integration: .taskfactory/logs/TF-001/pi-...log" and,
after adding the directory to `.git/info/exclude`, with "unexpected runtime
path .taskfactory/logs". Only the claim lock, the integration lock and the
evidence paths are allowed in `internal/integrate/integrate.go`. Allow
`.taskfactory/logs/` the same way and add a test.

## Notes

Each refusal also appends a FAILED integration evidence record (attempts 1 and
2 in the run), which is noise for a harmless untracked file.

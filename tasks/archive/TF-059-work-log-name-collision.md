# TF-059: Avoid log name collisions within one second

## Goal

Two `taskfactory work` runs for the same task and adapter that start in the
same second must both get a log file. The second run must not refuse with
"refusing to overwrite existing log".

## Dependencies

- TF-055
- TF-056

## Scope

`workprompt.LogPath` names a log `<adapter>-<UTC>.log` with a one-second UTC
stamp. `OpenLog` creates the file with `O_EXCL`, so a second open in the same
second fails. Keep the documented layout
`.taskfactory/logs/<ID>/<adapter>-<UTC>.log`, the one-second UTC stamp and the
never-overwrite rule. Change only the collision case:

1. `LogPath` stays pure and unchanged. It still returns the base name.
2. `OpenLog` takes the base path. If that file exists, it tries the next free
   numeric suffix before the extension, starting at 2, for example
   `pi-20261009T113550Z_2.log` and then `_3`. Each attempt uses `O_EXCL`, so
   concurrent opens never overwrite each other.
3. The suffix separator is `_`, not `-`. Under byte order `-` (0x2D) sorts
   before `.` (0x2E), so `-2.log` would sort before `.log`. `_` (0x5F) sorts
   after `.`, so every suffixed name sorts after the base name of the same
   second. This is the one departure from the example in the owner decision.
4. The attempts are bounded at 1000. After the last attempt `OpenLog` returns
   an error and creates nothing.
5. `OpenLog` returns the file whose `Name()` is the path actually used.
6. `internal/work` prints the path of the returned file, not the pre-computed
   `LogPath` value, in every `log:` line.

## Constraints

Standard library only. Never write credentials or environment values into a
log. Never override Git signing in product code; test fixtures set
`commit.gpgsign=false` in their own repository config only. Do not stage or
commit the log directory. The existing tests change only where they assert
the old refusal on a second open of the same path, and nothing else in them
changes.

## Success criteria

### C1: A second and third open in one second get suffixes

Check: go test ./internal/workprompt -run OpenLogSuffix -v

### C2: Concurrent opens in one second all succeed with distinct files

Check: go test ./internal/workprompt -run OpenLogConcurrent -v

### C3: Exhausting the suffix bound returns an error

Check: go test ./internal/workprompt -run OpenLogExhausted -v

### C4: Back-to-back runs in one second both succeed with different log paths

Check: go test ./internal/work -run BackToBack -v

### C5: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each C-check with `-v` and confirm a PASS line. Then run `bin/test` and
`bin/lint`, and `gofmt -l .`, and report each exit code. In a scratch
repository built with the real binary, run init, promote a tiny task, claim
it, put a fake adapter on PATH, and run `work` twice within the same second.
Paste both printed log paths and `ls .taskfactory/logs/TF-001`.

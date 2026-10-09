# Quick start verbatim rerun

Date: 2026-10-09. Binary: `taskfactory version 4930b7d` (README step 1,
`PREFIX=$(mktemp -d) bin/install`). Only README.md was read.

Worker: the real local model `Ornith-1.5-35B-A3B-MLX-4bit` via adapter
`pi` (server 127.0.0.1:8000 confirmed up). Worker wall time: 33 s.
The worker committed its own work (`2f109b4`).

Workarounds: 1.

The run is therefore not a success under the zero-workaround rule.

## Steps

| Step | Command                                     | Exit | Result                 |
| ---- | ------------------------------------------- | ---- | ---------------------- |
| 1    | `bin/install` (PREFIX temp)                 | 0    | installed, PATH hint   |
| 2    | `taskfactory init`                          | 0    | config and 5 dirs      |
| 3    | `validate tasks/inbox/TF-001-*.md`          | 0    | 1 file valid           |
| 4    | `promote TF-001`                            | 1    | Dependencies: use None |
| 4b   | same, after workaround W1                   | 0    | promoted, committed    |
| 5    | `claim TF-001 --owner you`                  | 0    | branch and worktree    |
| 6    | `work TF-001 --adapter pi ... --timeout 8m` | 0    | 33 s, committed        |
| 7    | `verify TF-001`                             | 0    | PASS (4 checks)        |
| 8    | `integrate TF-001`                          | 0    | integrated             |
| 9    | `check-main`                                | 0    | ok                     |
| 10   | `status`                                    | 0    | archive: 1             |
| F1   | TF-002 promote, claim                       | 0    | ok                     |
| F2   | `work TF-002` with fake pi                  | 1    | preflight refused      |
| F3   | `fail TF-002 --outcome BLOCKED`             | 0    | moved to failed        |

The fake `pi` (a script on PATH exiting 3) was a stand-in only for the
fail branch. The work refused at preflight (it lists models first), so
the 1s timeout was never reached; nothing was launched and the task
stayed active until `fail`.

## Workarounds

W1. README step 3 lists the headings but not the Dependencies format. I
wrote `None.`; `validate` accepted it, `promote` refused:

```text
taskfactory promote: refusing to promote TF-001: ...: Dependencies:
use None or - TF-NNN lines
```

Expected: validate and README agree. Fix: changed `None.` to `None`.

## README defects

1. Dependencies format unstated and validate/promote disagree (W1).
   Inbox: TF-062.
2. After a successful loop `git status --short` is not clean, and
   `init` writes no `.gitignore`. The README mentions only the log
   directory as untracked. Inbox: TF-063.

Minor, not counted: step 2 says edit `[verification]` but gives no
example of the TOML keys (`worker`, `integration`); I took them from the
generated file.

## Scratch repo after integrate (TF-001)

```text
git status --short
?? .taskfactory/claim.lock
?? .taskfactory/evidence/
?? .taskfactory/integration-evidence/
?? .taskfactory/integration.lock
?? .taskfactory/logs/
?? .taskfactory/worktrees/

git log --oneline
fae3f63 chore(tasks): archive integrated TF-001
2f109b4 Fix greeting: return Hello
dc0af7c docs: promote TF-001 to ready
c762748 taskfactory config
ef05d90 initial: failing greeting
```

After the fail branch the log also has `bd4db22 docs: promote TF-002 to
ready` and `7215782 docs: mark TF-002 failed`.

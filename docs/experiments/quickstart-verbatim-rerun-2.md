# Quick start verbatim rerun 2

Date: 2026-10-09. Binary: `taskfactory version ec686e0` (built from the
report worktree at main ec686e0 via `PREFIX=<mktemp -d> bin/install`).
Tester knew only README.md. Scratch repo under `mktemp -d`, signing off.

## Result

Workarounds: 0. The whole loop completed following the README verbatim.
Worker: the real local model `Ornith-1.5-35B-A3B-MLX-4bit` through
`--adapter pi` (server confirmed up with `curl /v1/models`). Wall time
of the real `work` run: 20 s (worker commit e50d38b made by the worker).

## Steps

| Step                      | Command                                | Exit | Result                     |
| ------------------------- | -------------------------------------- | ---- | -------------------------- |
| install                   | `PREFIX=<tmp> bin/install`             | 0    | binary built, PATH hint    |
| init                      | `taskfactory init`                     | 0    | config, 5 dirs, .gitignore |
| commit                    | `git add` config + .gitignore, commit  | 0    | clean                      |
| plan                      | write `tasks/inbox/TF-001-...md`       | 0    | README sample verbatim     |
| promote                   | `taskfactory promote TF-001`           | 0    | moved to ready             |
| claim                     | `taskfactory claim TF-001 --owner you` | 0    | branch, worktree           |
| work                      | `work TF-001` (pi, Ornith 35B, 8m)     | 0    | worker committed, 20 s     |
| verify                    | `taskfactory verify TF-001`            | 0    | PASS (2 checks)            |
| integrate                 | `taskfactory integrate TF-001`         | 0    | integrated                 |
| check-main                | `taskfactory check-main`               | 0    | ok                         |
| status                    | `taskfactory status`                   | 0    | archive: 1                 |
| TF-002 plan/promote/claim | same commands                          | 0    | active                     |
| stall                     | `work TF-002` (--timeout 1s)           | 1    | worker stalled             |
| fail                      | `fail TF-002 --outcome BLOCKED`        | 0    | in failed/                 |

`git status --short` after integrate (and check-main): empty.
`git status --short` after the fail branch: empty.

## README defects

1. Nothing says how to provoke a stall or non-zero worker exit for the
   fail branch. I used a 1 s `--timeout`, which worked (exit 1, prints a
   suggested `fail` command), but a newcomer must guess it.
2. After `integrate` and `fail`, the worktrees and branches
   `task/TF-001` and `task/TF-002` remain (`git worktree list`); the
   README does not say they stay or how to remove them.

Minor, not counted: `verify` prints "2 checks" for a task with one
Check line; the README does not explain which checks run.

## Final scratch git log

```text
d8237a6 docs: mark TF-002 failed
21a2953 docs: promote TF-002 to ready
b71c0e3 plan TF-002
a920fa9 chore(tasks): archive integrated TF-001
e50d38b Fix greeting to return Hello
44b3d45 docs: promote TF-001 to ready
331f2d2 chore: taskfactory config
b9bb177 init
```

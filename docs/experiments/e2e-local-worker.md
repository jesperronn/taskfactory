# End-to-end run with a local worker

Date: 2026-10-09 (UTC). Binary: `bin/install` from this branch
(`taskfactory version 5c24594-dirty`, installed with PREFIX into a scratch
`bin/`). The README quick start was followed in order.

The worker was a REAL local model, not a stand-in: adapter `pi`, model
`Ornith-1.5-35B-A3B-MLX-4bit` at the oMLX server 127.0.0.1:8000 (checked
first with `curl -s -m 3 http://127.0.0.1:8000/v1/models`, which listed it).
`work` passes `pi --provider omlx --model M --thinking low --no-session -p`.

Scratch project: `mktemp -d` Git repository (`git init -b main`), module
`example.com/hello`, one commit `38ffdcc`, a test that fails because
`Greeting()` returns `hello` instead of `hello, world`. Signing: the repo
config set `commit.gpgsign=false`, with `GIT_CONFIG_GLOBAL=/dev/null` and
`GIT_CONFIG_SYSTEM=/dev/null`. TaskFactory itself passed no signing flags.
Nothing was pushed; the real home and this repository's `.taskfactory` were
not touched.

## Steps

`tf` is the installed binary, run in the scratch project.

| #   | Command                                        | Exit | Result                 |
| --- | ---------------------------------------------- | ---- | ---------------------- |
| 1   | PREFIX=scratch/bin bin/install                 | 0    | installed              |
| 2   | tf init                                        | 0    | silent, config written |
| 3   | tf validate tasks/inbox/TF-001 file            | 0    | 1 valid                |
| 4   | tf promote (task staged with config, my error) | 1    | staged paths           |
| 5   | commit config only; tf promote TF-001          | 0    | 838e9cf                |
| 6   | tf claim TF-001 --owner e2e                    | 0    | claimed                |
| 7   | tf work TF-001 --adapter pi --model M          | 1    | no worker doc          |
| 8   | MANUAL: commit worker doc, rebase worktree     | 0    | 900de41                |
| 9   | tf work TF-001 --adapter pi --timeout 8m       | 0    | commit 8a6db5a         |
| 10  | tf verify TF-001                               | 0    | PASS, 2 checks         |
| 11  | tf integrate TF-001                            | 1    | logs dir blocks        |
| 12  | logs dir in .git/info/exclude; integrate       | 1    | runtime path           |
| 13  | MANUAL: mv .taskfactory/logs out of repo       | 0    | log kept               |
| 14  | tf integrate TF-001                            | 0    | integrated             |
| 15  | tf check-main                                  | 0    | check-main: ok         |
| 16  | tf status                                      | 0    | archive: 1             |

In step 9, M is `Ornith-1.5-35B-A3B-MLX-4bit`. In step 7 the refusal was
"read docs/worker-instructions.md ... no such file or directory".

Worker wall time (step 9, `time`): 39.5 s real (the first attempt, step 7,
refused in 0.03 s). Step 8 is a manual fix and step 13 a manual workaround;
neither is part of the product.

## Key output

Worker log, kept at
`<scratch>/proj/.taskfactory/logs/TF-001/pi-20261009T111638Z.log` until step
13 moved it to `<scratch>/logs-saved/TF-001/`. Its header (trimmed):

```text
Adapter: pi   Model: Ornith-1.5-35B-A3B-MLX-4bit   Timeout: 8m0s
State: exit   Exit code: 0
```

`work` printed:

```text
worker exited 0 (a claim by the worker, not a pass)
worker commit: 8a6db5a hello: return "hello, world" from Greeting
next: taskfactory verify TF-001
```

The worker changed only `hello.go` (1 line). Its own report said `go test`
exited 0 and that there was no `bin/test`; `verify` then recorded PASS with
both the task check C1 and the worker check at exit 0.

Final history of the scratch main:

```text
7f42701 chore(tasks): archive integrated TF-001
8a6db5a hello: return "hello, world" from Greeting
900de41 docs: add worker instructions (manual fix, work needs it)
838e9cf docs: promote TF-001 to ready
b126f06 chore: add taskfactory config
38ffdcc initial
```

The integration commit is `7f42701` (the archive commit); the worker's
commit `8a6db5a` was fast-forwarded into main by the same step.

## Friction and defects

| ID  | Kind   | Observation                                 | Filed  |
| --- | ------ | ------------------------------------------- | ------ |
| F1  | defect | `work` needs docs/worker-instructions.md    | TF-060 |
| F2  | defect | `integrate` refuses on `.taskfactory/logs/` | TF-061 |
| F3  | doc    | stage config only before promote (my error) | README |
| F4  | note   | claim base stays pre-rebase in evidence     | none   |
| F5  | note   | `init` makes empty tasks/ dirs Git ignores  | none   |

F2 also appended a FAILED integration evidence record per refusal. F4: the
evidence `changed_files` listed my manual worker-instructions file too.

Quick start verdict: it could not be followed verbatim without manual fixes
(steps 8 and 13), so the TF-035 "verbatim" criterion is not yet met; TF-060
and TF-061 are the remaining gaps. TF-020 (CI run) is untouched.

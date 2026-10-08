# End-to-end dry run of the TaskFactory loop

Date: 2026-10-08 (UTC). Binary: `bin/build` at `a587e23`
(`taskfactory --version` printed `taskfactory version a587e23`).

Scratch project: a fresh `git init -b main` repository under `mktemp -d`
with a tiny Go module (`hello, world` style greeting, a test, `bin/test`
and `bin/lint`), one commit `f4599d6`. `GIT_CONFIG_GLOBAL` and
`GIT_CONFIG_SYSTEM` were set to `/dev/null`. Nothing was pushed. The real
home directory and this repository's `.taskfactory` were not touched.

## Docs read

`AGENTS.md`, `docs/worker-instructions.md`, `docs/protocol-v1.md`,
`docs/config-v1.md`, `skills/taskfactory/SKILL.md`, `README.md`. The
skill and the protocol disagree about `promote` (see D1).

## Step table

Exit codes are those of the step's command. Commands are abbreviated;
`tf` is the built binary, run from the scratch project.

| #   | Command                   | Exit | Result                      |
| --- | ------------------------- | ---- | --------------------------- |
| 1   | git worktree add          | 0    | worktree ../tf-e2e created  |
| 2   | bin/build DIR/bin-tf      | 0    | built at a587e23            |
| 3   | tf --help                 | 0    | lists promote, check-main   |
| 4   | go test ./... (scratch)   | 0    | ok example.com/hello        |
| 5   | bin/test (scratch)        | 0    | ok example.com/hello        |
| 6   | bin/lint (scratch)        | 0    | no output, gofmt clean      |
| 7   | git commit (scratch)      | 0    | f4599d6 initial commit      |
| 8   | tf --version              | 0    | version a587e23             |
| 9   | tf init                   | 0    | no output, config written   |
| 10  | tf status                 | 0    | five states, inbox: 0       |
| 11  | tf validate /outside/path | 2    | refused: not in tasks dir   |
| 12  | tf validate inbox file    | 0    | 1 task file(s) valid        |
| 13  | tf promote TF-001         | 0    | promoted, commit 8c044ba    |
| 14  | tf promote --help         | 0    | usage; commits two paths    |
| 15  | tf validate (whole tree)  | 0    | inbox and ready valid       |
| 16  | tf status                 | 0    | ready: 1                    |
| 17  | tf validate ready file    | 0    | 1 task file(s) valid        |
| 18  | git commit config, tasks  | 0    | 9492b87 config commit       |
| 19  | tf claim TF-001 --owner   | 0    | claimed TF-001 for e2e      |
| 20  | sed -i (BSD, no suffix)   | 1    | my error, not a product bug |
| 21  | bin/test (no edit made)   | 0    | passes on unchanged code    |
| 22  | edit main.go, main_test   | -    | file editor, not shell      |
| 23  | git commit (no changes)   | 1    | my error, nothing staged    |
| 24  | bin/test, then commit     | 0    | 275a37e feat commit         |
| 25  | tf verify TF-001          | 0    | silent; PASS evidence       |
| 26  | cat evidence jsonl        | 0    | C1, C2, worker all exit 0   |
| 27  | tf verify --help          | 0    | exit codes documented       |
| 28  | tf integrate TF-001       | 0    | integrated; archive 8a9bee3 |
| 29  | git merge-base --is-anc.  | 0    | 275a37e is in main          |
| 30  | tf check-main             | 0    | check-main: ok              |
| 31  | tf check-main --help      | 0    | usage; reruns checks        |
| 32  | tf status                 | 0    | archive: 1, ready/active: 0 |

Steps 20 and 23 are my own mistakes, not product defects. Step 20 used
macOS `sed -i` without a backup suffix. The error text was
`sed: 1: "main.go": invalid command code m`. Step 23 failed because the
`sed` chain had stopped. I then edited the files with the file editor.

## Key outputs

Integration record for TF-001 (`integration-evidence`, abridged to the
fields that matter):

```text
outcome=PASS stage=archive
main_before=9492b87  verified=275a37e  main_after=8a9bee3
```

Main history after integration:

```text
8a9bee3 chore(tasks): archive integrated TF-001
275a37e feat: greet the world (TF-001)
9492b87 chore: add taskfactory config
8c044ba docs: promote TF-001 to ready
f4599d6 chore: tiny hello module
```

The worker evidence record has `outcome` `PASS`, `result_commit`
`275a37e`, and three checks, all exit 0. `verify` printed nothing.

## Findings

Priority order.

- D1 (high, doc drift): `SKILL.md` says lifecycle commands, including
  promote, are not implemented (TF-033) and that a planner moves the
  file by hand. The binary has `promote`, which also commits. The
  `SKILL.md` command table lacks `promote` and `check-main`. Also
  `README.md` and the skill disagree on how a ready task is produced.
- D2 (medium, UX): `verify` prints nothing on success. The worker has to
  read `.taskfactory/evidence/<ID>.jsonl` to learn the outcome. The
  `worker-instructions.md` step "Read the output" has no output to read.
- D3 (medium, docs): the expected "evidence commit" does not exist.
  Worker and integration evidence live in untracked `.taskfactory/`
  files. After integrate, `git status` shows four untracked
  operational paths. No doc states plainly that evidence is not in Git
  history.
- D4 (low, docs): `init` leaves `.taskfactory/config.toml` untracked and
  prints nothing about it. `SKILL.md` step 4 says to commit it before
  claiming. `promote` commits only the task paths, so the commit order
  is easy to get wrong. I did not test `claim` with an untracked config.
- D5 (low, docs): `validate <path>` refuses files outside `tasks/`
  with exit 2. The message is clear, but no doc says so.

Blockers: none. Failure paths of `verify` and `integrate` were not
exercised; this run covered only the happy path.

## Tasks filed

One inbox task per real defect: TF-050 (D1), TF-051 (D2), TF-052 (D3).
D4 and D5 are doc notes only and were not filed.

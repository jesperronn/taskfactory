# TF-034 skill adoption record

This record shows the TaskFactory skill used in a fresh scratch repository. The
binary was built from commit 06b50e9 with `bin/build` into a `mktemp -d`
directory. All repository commands used `GOTOOLCHAIN=local` and
`GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null`. The scratch
repository had no TaskFactory files before `taskfactory init`.

## Run 1: skill text before fixes

Each block shows the command, its exit code, and its output verbatim.

### Build

```sh
bin/build "$TFBIN_DIR/taskfactory"
```

Exit code: 0

```text
built /var/folders/vc/ckfmj_3s2dz6wx2ld2hj_w4c0000gq/T/tmp.hRlf68iQ1i/taskfactory (version 06b50e9)
```

### Scratch repository

```sh
git init -q -b main && echo "# scratch" > README.md
git add README.md && git commit -q -m init
git log --oneline
```

Exit code: 0

```text
fe588da init
```

### taskfactory init

```sh
taskfactory init
```

Exit code: 0. No output.

Resulting `.taskfactory/config.toml`:

```toml
protocol_version = 1

[workers]
max_parallel = 4

[git]
use_worktrees = true
worktree_root = ".taskfactory/worktrees"
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = ["go test ./..."]
integration = ["go test ./...", "go vet ./..."]
```

Created directories: `tasks/active`, `tasks/archive`, `tasks/failed`,
`tasks/inbox`, `tasks/ready`.

### Task file

The file `tasks/ready/TF-001-hello.md` was written by hand. Run 1 used the
template in `docs/task-format-v1.md`, which the scratch repository does not
have; the skill did not yet include one (see fix 3). It has the seven required
headings, one criterion `### C1: Hello file exists` and `Check: test -f hello.txt`.

### Validate the task file

```sh
taskfactory validate tasks/ready/TF-001-hello.md
```

Exit code: 0

```text
1 task file(s) valid
```

### Validate the tree

```sh
taskfactory validate
```

Exit code: 0

```text
inbox and ready tasks are valid
```

### Status (not in the worker steps, run to check the skill's table)

```sh
taskfactory status
```

Exit code: 0

```text
inbox: 0
ready: 1
active: 0
failed: 0
archive: 0
```

### Claim

```sh
taskfactory claim TF-001 --owner worker
```

Exit code: 0

```text
claimed TF-001 for worker
```

Claim block written to `tasks/active/TF-001-hello.md`:

```text
## Claim

Owner: worker
Branch: task/TF-001
Worktree: /var/folders/vc/ckfmj_3s2dz6wx2ld2hj_w4c0000gq/T/tmp.NeBzbu6Qnu/.taskfactory/worktrees/TF-001
Base commit: fe588daefb3cc08078e0fc95eb0133d7bfb8cd7d
Started at: 2026-10-08T22:41:01Z
```

### Second claim (must fail)

```sh
taskfactory claim TF-001 --owner worker
```

Exit code: 1

```text
taskfactory claim: task TF-001 is not eligible: no ready task file exists for this ID
```

### Worktree check

```sh
git worktree list
ls <worktree>/tasks/
```

`git worktree list` lists `.taskfactory/worktrees/TF-001` on `task/TF-001` at
`fe588da`. `ls` of `<worktree>/tasks/` fails with exit 1 and
`No such file or directory`. The worker's checkout therefore has no contract.
The cause is that the ready task was never committed, and worktrees are created
from `HEAD`.

## Run 2: after the skill fixes

The same steps were repeated in a second scratch repository. This time the
config was committed, the verification commands were set to the scratch
project's own commands, and the contract was committed before claim.

Results (condensed; exit codes are from each command):

```text
init exit=0
1 task file(s) valid                       (validate, exit=0)
claimed TF-001 for worker                  (claim, exit=0)
```

`git worktree list` shows `.taskfactory/worktrees/TF-001` on `task/TF-001` at
`079dbbc`. Its `tasks/ready/TF-001-hello.md` exists, so the worker can read the
contract. The Claim block names the same worktree path as the claim.

## Fixes made to skills/taskfactory/SKILL.md

1. Added `taskfactory check-main` to the command table. It exists in the binary
   but the skill did not list it.
2. Rewrote the "Not yet implemented" section. It said `bin/build` did not exist
   and that no worker adapter existed. Both were false: `bin/build` exists
   (TF-031) and adapters exist as `internal/adapter/{claude,omp,pi}` packages
   with no CLI launch command.
3. Made the skill self-contained. It told workers to read `docs/*.md` in the
   project, which an adopting project does not have. Added the ready-contract
   template and its rules.
4. Added setup steps: a Git repository with a commit, `init` writes Go-specific
   `[verification]` commands that must be replaced, and the scaffold and
   contract must be committed before claim (proved by run 1 vs run 2).
5. Corrected the claim step. The command prints only a confirmation, not the
   worktree, so the worker reads the path from the Claim block.
6. Noted that `.taskfactory/` operational files and `tasks/active/` show as
   untracked or changed after claims and must not be committed by hand.
7. Made the worker's `bin/test` and `bin/lint` step conditional on the project
   providing them.
8. Fixed the table layout so the claim row fits on one line.

## Not run from the skill

`taskfactory --help`, `taskfactory check-main --help` and
`taskfactory verify --help` were run from the repository to check flags. They
are not part of the adoption steps.

# TaskFactory

**TaskFactory turns software work into explicit, verifiable tasks that coding
agents can execute safely in parallel.**

A capable planner defines **what must be done and how success will be proven**.
Workers independently implement those tasks, test their own work, diagnose
failures, and keep fixing until the predefined success criteria pass.

TaskFactory coordinates the process using a small CLI, Git worktrees, task
files, and deterministic verification.

```text id="ykw00l"
                  ┌─────────────┐
                  │   PLANNER   │
                  └──────┬──────┘
                         │ refine / split / specify
                         ▼
 📥 inbox/           ✅ ready/            🔧 active/
 ideas & rough  ──► executable tasks ──► claimed by workers
 requirements              │                   │
                           │              ┌────┼────┐
                           │              ▼    ▼    ▼
                           │             W1   W2   W3   W4
                           │              │    │    │    │
                           │             isolated Git worktrees
                           │              │    │    │    │
                           │              └────┼────┘
                           │                   │
                           │            self-verification
                           │                   │
                           │                   ▼
                           │           integration queue
                           │                   │
                           │                   ▼
                           │             INTEGRATOR
                           │                   │
                           │          rebase latest main
                           │                   │
                           │               verify
                           │                   │
                           │          git merge --ff-only
                           │                   │
                           │            ┌──────┴──────┐
                           │            │             │
                           │          success       failure
                           │            │             │
                           │            ▼             ▼
                           │       📦 archive/    💥 failed/
                           │
                           └──── dependencies determine
                                 what becomes ready
```

## The idea

Coding agents are much more useful when they are given small tasks with an
objective definition of "done."

Instead of telling an agent:

> Implement authentication.

TaskFactory aims to give it something closer to:

```text id="2wravc"
Goal
  Reject expired authentication assertions.

Constraints
  Do not change the public API.
  Do not upgrade dependencies.

Success criteria

  SC1
    Expired assertions are rejected.

    Verify:
      run the targeted authentication tests

  SC2
    Existing authentication still works.

    Verify:
      run the complete authentication test suite
```

The worker doesn't decide when the task is finished.

The **task contract defines when it is finished**.

---

# Quick start

This walks one small task through the whole loop with a local worker. Run it in
a Git repository with at least one commit on `main` (or on the branch
`git.base_branch` names). Every step is a command that exists today, including
`requeue` for the failed branch.

1. Install. From a TaskFactory checkout run `bin/install`, which builds
   `taskfactory` (and the short `tf` alias, which works anywhere `taskfactory`
   does; the full name is still preferred in docs and prompts) and prints a
   hint if its directory is not on `PATH`.
2. Init. In your project run `taskfactory init [path]`; the path may be any
   directory inside the project and defaults to the current directory. It writes
   `.taskfactory/config.toml`, the five `tasks/` directories and a
   `.gitignore` (or appends to yours) that lists the runtime paths: the two
   lock files, `evidence/`, `integration-evidence/`, `logs/` and `worktrees/`
   under `.taskfactory/`. It never ignores the config and never edits other
   lines. It prints nothing. Edit `[verification]` in the config to your own
   test commands, then commit the config and the `.gitignore` together before
   the first claim: claim and integrate need the config tracked, and an
   uncommitted `.gitignore` would show as untracked. `work` also
   uses `docs/worker-instructions.md` from the claimed worktree when it
   exists. When it does not, the prompt carries built-in worker instructions
   and says so; `init` does not write the file.

   To keep the five task folders somewhere else under the state root, for
   example `.taskfactory/tasks`, set the top-level `tasks_dir` key in
   `.taskfactory/config.toml`; without it they are `tasks/`. `init` writes no
   key and creates the folders where an existing config puts them. See
   `docs/config-v1.md`.

   To keep task state outside the project, run
   `taskfactory init [path] --state-dir <dir>`. It writes the five `tasks/`
   directories and `.taskfactory/config.toml` (with `project_root` naming the
   code repository) in `<dir>` and nothing in the project; later commands find
   it through `--state-dir <dir>` or `TF_STATE_DIR`. If `<dir>` is a Git
   repository, `promote`, `fail`, `requeue` and the integrate archive step
   commit their moves there (`git -C <dir>`, same messages, your git config;
   signing is never overridden), the "tracked in HEAD" guards look at that
   repository, and `init` adds the runtime paths to its `.gitignore`. If `<dir>`
   is a plain folder, moves are plain renames. Integrate records a `MERGED`
   marker in `<dir>/.taskfactory/integration-evidence/` after the project
   fast-forward; if the archive move or state commit then fails or crashes, the
   next `integrate <ID>` reads the marker, finishes the archive and commit, and
   never merges twice.
3. Plan. Run `taskfactory new "Fix the greeting"` to write a skeleton to
   `tasks/inbox/` with the next free ID (archive included), every ready heading
   and `TODO` markers; `promote` refuses the file until every `TODO` is replaced.
   Or write `tasks/inbox/TF-001-<slug>.md` by hand with a `# TF-001: title`
   heading and the ready headings Goal, Dependencies, Scope, Constraints,
   Success criteria (each `### CN:` followed by one `Check:` line) and
   Verification. Dependencies is exactly `None`, or one `- TF-NNN` line per
   archived task and nothing else, with no trailing period: `None.` is
   refused. `validate` checks inbox files loosely on purpose, so a passing
   `taskfactory validate tasks/inbox/<file>` does not promise that `promote`
   accepts the file; `promote` applies the full ready contract. A complete
   minimal task:

   ```markdown
   # TF-001: Fix the greeting

   ## Goal

   Greet() returns "Hello".

   ## Dependencies

   None

   ## Scope

   Edit greeting.go only.

   ## Constraints

   Keep the change small.

   ## Success criteria

   ### C1: The greeting test passes

   Check: go test ./... -run TestGreet -v

   ## Verification

   Run the check and report its exit code.
   ```

4. Promote. `taskfactory promote TF-001` moves the file to `tasks/ready/`
   and commits only that move.
5. Claim. `taskfactory claim TF-001 --owner you` creates the branch
   `task/TF-001` and the worktree `.taskfactory/worktrees/TF-001`.
6. Work. `taskfactory work TF-001 --adapter pi --model <id> --timeout 8m`
   launches a local worker in that worktree. Adapter and model are always
   explicit. It prints the log path (under `.taskfactory/logs/TF-001/`) first.
   Exit 0 only means the worker claims it is done. If the worker did not
   commit, commit in the worktree yourself. The log directory is
   ignored; `integrate` never stages it.
7. Verify. `taskfactory verify TF-001` reruns the task checks in the worktree
   and appends evidence to `.taskfactory/evidence/TF-001.jsonl`. Only this
   evidence counts, not the worker's report.
8. Integrate. `taskfactory integrate TF-001` rebases the branch onto main
   (the `git.base_branch` branch, default `main`), verifies again,
   fast-forwards it and archives the task.
9. Check main. `taskfactory check-main` reruns main's verification. After
   a successful loop `git status --short` is empty.

`integrate` and `fail` keep the task worktree and its branch. After either,
`git worktree list` still shows `.taskfactory/worktrees/TF-001` on
`task/TF-001`, and `git branch` still lists `task/TF-001`. Nothing removes
them. Remove them yourself when you no longer need them:

```sh
git worktree remove .taskfactory/worktrees/TF-001
git branch -d task/TF-001
```

`git branch -d` deletes the branch only when Git sees it as merged. After
`integrate` the branch is merged into main, so it deletes.

If the worker cannot finish (a stall, a refusal, a failed check that it
cannot repair), run `taskfactory fail TF-001 --outcome BLOCKED --reason "..."`
(or `FAILED` after a failed verify attempt). It moves the task to
`tasks/failed/` and commits only that move. To retry, run
`taskfactory requeue TF-001` (or `--to inbox`). It moves the unclaimed task
back to `tasks/ready/` and commits only that move, and it resets the attempt
counter by renaming the evidence file aside to
`.taskfactory/evidence/TF-001.attempts-<N>.jsonl`, and the next attempt starts
at 1. A failed task that still has a Claim block, which is every task failed
after a claim, is flagged for a human decision and `requeue` refuses it with
exit 1. `taskfactory requeue --help` lists such claimed failed IDs. To decide,
run `taskfactory requeue TF-001 --discard-claim` (add `--to inbox` if needed).
It removes the whole Claim block from the task file, then requeues as above.
The commit message body records the discarded claim (owner, branch, base
commit and worktree path), so nothing is lost. On a task without a Claim block
the flag is accepted and nothing is discarded.

After the requeue commit, `requeue` also removes the task's old worktree and
branch, so that `taskfactory claim` can create them again (otherwise the claim
fails with `branch task/TF-001 already exists`). It removes the ones recorded in
the Claim block, or the conventional `.taskfactory/worktrees/TF-001` and
`task/TF-001` when there is no Claim block. It does this only when it is safe:
the worktree has no changes and no untracked files, the branch has no commits
that are not already on the local base branch, `main` by default (a branch still
at its claim base commit counts), and the worktree is inside
`.taskfactory/worktrees` and is not the main checkout. These checks run before
anything changes. If one fails, `requeue` exits 1, changes nothing and prints
the commands below. The cleanup runs `git worktree remove` and `git branch -d`,
never with force and never with `-D`. A worktree or branch that is already gone
is not an error. If the cleanup fails after the commit, `requeue` exits 1 and
says the task was requeued and what it left behind.

Unmerged worker commits therefore always need your decision. A failed task
usually carries worker commits, so the refusal is the common case, and the
manual commands remain the way. Inspect the commits with
`git log --oneline main..task/TF-001`, then remove the worktree and branch
yourself and run `requeue` again:

```sh
git worktree remove <worktree path from the Claim block>
git branch -d <branch from the Claim block>
```

Use `git branch -D <branch>` instead only when you decide to discard the
unmerged commits, and `git worktree remove --force` only to discard changes
in the worktree.

To see the fail branch for yourself, make a worker stall in your scratch repo
with a one-second timeout. Claim a ready task, then run:

```sh
taskfactory work TF-001 --adapter pi --model <id> --timeout 1s
```

It prints the log path, then `worker stalled: timeout exceeded` with the
adapter's progress note, and exits 1. It also prints a line that starts
`suggestion, not run: taskfactory fail TF-001 --outcome BLOCKED`. Run that
fail command yourself; `work` never fails a task.

Automatic versus a human decision:

- Automatic once started: `claim` (branch and worktree), the worker's own
  edits, `verify`, the rebase, recheck and archive inside `integrate`, and
  `check-main`.
- A human decides: what to plan, `promote`, which adapter and model `work`
  uses, whether to commit what the worker left, `fail` (any step that gives
  up on or retries work), `integrate`, and what to do when `check-main` fails.
- `work` never commits, verifies, integrates or fails anything. TaskFactory
  never overrides commit signing; see Commit signing below.

---

# The task lifecycle

Every task exists in one of five directories:

```text id="e7cnjg"
tasks/
├── inbox/
├── ready/
├── active/
├── failed/
└── archive/
```

The filesystem is deliberately used as the task-state store.

There is no TaskFactory database or server.

## 📥 `inbox/` — needs planning

`inbox` contains work we know about but that an autonomous worker should **not
start yet**.

Examples:

- an idea;
- a feature request;
- a bug report;
- a large piece of work;
- an ambiguous requirement;
- a task without tests or success criteria.

For example:

```text id="utbflm"
Improve authentication error handling.
```

That may be useful work, but it isn't a worker contract yet.

The planner investigates and refines it.

It may turn one inbox item into several independent tasks.

---

## ✅ `ready/` — safe to execute

A task enters `ready` only when it is sufficiently specified for an autonomous
worker.

This distinction is fundamental:

```text id="ttkgkn"
inbox
    = we know we want this

ready
    = a worker can safely start this now
```

A ready task should contain:

- a clear goal;
- explicit scope;
- relevant constraints;
- dependencies;
- predefined success criteria;
- verification for those criteria.

A worker should **not need to perform project-level planning** before starting a
ready task.

This guarantee is what makes parallel execution possible.

---

## 🔧 `active/` — being implemented

When a worker claims a ready task, TaskFactory moves it to `active`.

Claiming is atomic: two workers must never successfully claim the same task.

TaskFactory assigns the worker its own:

```text id="fpttrz"
task
branch
Git worktree
base commit
```

For example:

```text id="dwp62v"
main

task/TF-101 ───── worktree-1
task/TF-102 ───── worktree-2
task/TF-103 ───── worktree-3
task/TF-104 ───── worktree-4
```

TaskFactory initially allows at most **four implementation workers** at once.

Each worker has an isolated filesystem, so workers do not edit the same
checkout.

---

# Workers verify their own work

TaskFactory does not use the factory as a remote test runner that repeatedly
tells an agent what went wrong.

The worker owns the complete implementation loop:

```text id="3rf6qi"
        ┌──────────────────────────┐
        │                          │
        ▼                          │
     implement                     │
        │                          │
        ▼                          │
      verify                       │
        │                          │
   ┌────┴────┐                     │
   │         │                     │
 PASS       FAIL                   │
   │         │                     │
   │      diagnose                 │
   │         │                     │
   │        fix                    │
   │         │                     │
   │         └─────────────────────┘
   │
   ▼
return evidence
```

If a test fails, the worker should inspect the failure, fix the implementation,
and run verification again.

It keeps working until either:

```text id="wsf8d7"
all success criteria pass
```

or:

```text id="rdckqa"
the allowed execution/retry budget is exhausted
```

---

# Workers cannot redefine success

The planner defines success **before implementation starts**.

A worker must not make its task pass by:

- removing a failing test;
- weakening a test;
- removing a success criterion;
- skipping required verification;
- silently changing the requirement;
- declaring that something is "good enough."

If the specification itself appears impossible or contradictory, that is a
legitimate result.

The worker reports the problem instead of rewriting its own contract.

---

# Workers return evidence

A worker does not merely report:

```text id="4v6qma"
Done!
```

It returns evidence.

Conceptually:

```text id="asdpb3"
Task: TF-101
Status: PASS

Changed:
  auth.go
  auth_test.go

Verification:

  SC1
    go test ./...
    exit 0

  SC2
    go vet ./...
    exit 0

Attempts: 2

Result commit:
  abc1234
```

TaskFactory treats this as a **proof package**.

---

# Integration is different from implementation

Workers execute in parallel.

Integration does not.

```text id="3p3u6n"
 W1 ───┐
 W2 ───┤
 W3 ───┼──► integration queue ───► one integrator ───► main
 W4 ───┘
```

This is intentional.

A task may have been correct when the worker started but another worker may have
changed `main` in the meantime.

Before integration, the candidate therefore has to prove itself against the
**current** repository.

---

# Integration process

The integrator performs approximately:

```text id="5o1tfa"
acquire integration lock
        │
        ▼
get latest main
        │
        ▼
rebase task onto main
        │
        ▼
run integration verification
        │
        ▼
git merge --ff-only
        │
        ▼
optional main verification
        │
        ▼
archive task
```

TaskFactory uses:

```bash id="ylrr77"
git merge --ff-only
```

If `main` changed while a candidate was waiting, the fast-forward merge cannot
silently hide that fact.

The candidate must rebase and prove itself again.

---

# Why verify after rebasing?

Consider two workers starting from the same commit:

```text id="3i6qxj"
             ┌── TF-101
main ────────┤
             └── TF-102
```

TF-101 finishes first:

```text id="g27m93"
main ─── TF-101
          \
           TF-102 (based on old main)
```

TF-102's tests may have passed in its original worktree.

That is no longer sufficient.

TaskFactory rebases TF-102:

```text id="6ev6wg"
main ─── TF-101 ─── TF-102
```

and runs verification again.

Only the rebased version may be integrated.

---

# When individually correct tasks conflict

Parallel development creates another interesting situation.

Suppose:

```text id="bdh6fo"
TF-101 passes independently  ✓
TF-102 passes independently  ✓

TF-101 + TF-102              ✗
```

TaskFactory should not blindly label TF-102 as bad.

Instead it can create a remediation task describing:

```text id="em9evl"
Related:
  TF-101
  TF-102

Known:
  TF-101 passed independently
  TF-102 passed independently

Failure:
  <integration test>

Goal:
  Make the implementations work together while
  preserving both original task contracts.
```

A stronger worker can then solve the interaction without throwing away valid
work.

---

# 💥 `failed/` — useful failure

A failed worker is not supposed to disappear with:

```text id="vbtowd"
couldn't do it
```

TaskFactory preserves useful information:

```text id="5azv0f"
original task
branch
last commit
attempts
modified files
successful checks
failed check
error output
worker diagnosis
```

The orchestrator can then decide to:

```text id="nm4lqh"
retry with another worker
        │
        ├──► stronger model
        │
        ├──► planner splits task
        │
        ├──► prerequisite task
        │
        └──► give up
```

This makes escalation much cheaper than starting again from scratch.

---

# 📦 `archive/` — accepted work

A task enters `archive` after successful integration.

The archive therefore becomes a useful record of:

- what was requested;
- why it was requested;
- how success was defined;
- what was changed;
- what verification passed;
- which commit implemented it.

`archive` means more than "the worker stopped working."

It means the work was accepted into the repository.

---

# Stop the line

TaskFactory assumes `main` should remain green.

If repository-level verification discovers that `main` is broken:

```text id="kexu04"
                 MAIN RED

workers             integration
continue                X
   │                     │
   │               STOP THE LINE
   │                     │
   │               remediation task
   │                     │
   │                 repair main
   │                     │
   │                 MAIN GREEN
   │                     │
   └──────────────► integration resumes
```

Implementation workers may continue because their worktrees are isolated.

But **ordinary integration stops immediately**.

Repairing `main` becomes the highest-priority integration concern.

---

# The four roles

TaskFactory separates four responsibilities.

## 🧠 Planner

The planner thinks about the work.

It:

- refines inbox items;
- investigates requirements;
- splits large tasks;
- identifies dependencies;
- identifies parallel work;
- defines constraints;
- defines success criteria;
- defines verification.

Its main question is:

> What exactly must be true before we can objectively say this task is finished?

---

## 🎛 Orchestrator

The orchestrator runs the factory.

It:

- manages lifecycle state;
- keeps at most four implementation workers active;
- assigns ready work;
- handles retries;
- escalates failed tasks;
- manages priorities;
- controls integration;
- invokes planning when necessary.

Much of this should eventually be deterministic CLI behavior rather than LLM
reasoning.

---

## 🔧 Worker

The worker implements one task.

It:

```text id="xg5y4n"
claim
  ↓
understand
  ↓
implement
  ↓
test
  ↓
diagnose
  ↓
repair
  ↓
prove success
```

Workers operate in isolated Git worktrees.

---

## 🚦 Integrator

The integrator protects `main`.

It:

- integrates one candidate at a time;
- rebases against latest main;
- runs integration verification;
- uses `git merge --ff-only`;
- archives successful tasks;
- stops integration when main becomes unhealthy.

The integrator should normally **not implement features**.

---

# Dependencies

Tasks may depend on other tasks.

For example:

```text id="vwtw0a"
TF-101
   │
   ├────► TF-103
   │
TF-102
   │
   └────► TF-104
```

TF-101 and TF-102 may execute in parallel.

TF-103 cannot become executable until its dependency is satisfied.

The planner determines dependencies.

The CLI enforces them.

---

# TaskFactory's own architecture

TaskFactory intentionally avoids becoming a large orchestration platform.

The source of truth is:

```text id="vqxl68"
┌─────────────────┬────────────────────────────────┐
│ Git             │ source-code state              │
│ filesystem      │ task lifecycle                 │
│ TOML            │ project configuration          │
│ task contracts  │ requirements + success         │
│ taskfactory CLI │ deterministic state changes    │
│ agents          │ reasoning + implementation     │
└─────────────────┴────────────────────────────────┘
```

There is initially:

- no database;
- no daemon;
- no server;
- no distributed queue;
- no cloud dependency.

---

# Configuration

Each initialized project has:

```text id="sp21fw"
.taskfactory/
└── config.toml
```

For example:

```toml id="p15m6j"
protocol_version = 1

[workers]
max_parallel = 4

[git]
use_worktrees = true
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = [
    "go test ./..."
]

integration = [
    "go test ./...",
    "go vet ./..."
]
```

Configuration is project policy.

Individual task requirements remain in task contracts.

---

# The CLI

TaskFactory is a small Go executable intended to live directly on `$PATH`.

The initial command direction is:

```text id="ft39l7"
taskfactory --help
taskfactory --version

taskfactory init [path] [--state-dir <dir>]
taskfactory status
taskfactory validate
taskfactory new "<title>"
taskfactory claim
taskfactory verify
taskfactory integrate
taskfactory check-main
taskfactory promote <ID>
taskfactory fail <ID> --outcome <FAILED|BLOCKED>
taskfactory requeue <ID> [--to ready|inbox] [--discard-claim]
taskfactory work <ID> --adapter <omp|pi|claude> --model <id>
taskfactory <command> --help
```

Every command ends with a hint about what to do next. `init` prints a
`Next steps:` block (review the config, commit it, write a task, promote it,
claim it). The other commands print one `next:` line on stdout after their normal
output, for example `next: taskfactory integrate TF-001` after a passing
`verify`. A failure with an obvious recovery prints its `next:` line on stderr
after the error. Usage errors print no hint, and exit codes and
machine-readable lines are unchanged. Each command's `--help` has a `Next step:`
line.

Every command accepts `--verbose` and `--dry-run`, before or after its other
arguments. `--verbose` adds `verbose: ` lines on stderr (the Git commands run,
the paths touched, the project root) and never changes stdout or the exit code;
credential values from the environment are redacted. `--dry-run` prints each
action as a `would: ` line, ends with `dry run: nothing changed`, and changes
nothing: no move, commit, worktree, evidence write or adapter launch. It
refuses what the real command refuses, with the same exit code.

```sh
taskfactory claim TF-001 --owner me --dry-run
# would: move task to tasks/active
# would: add worktree /path/to/project/.taskfactory/worktrees/TF-001
# dry run: nothing changed
taskfactory verify TF-001 --verbose   # same stdout, plus verbose: lines on stderr
```

`taskfactory fail` moves a claimed active task to `tasks/failed` and commits
only that move. Its last attempt evidence must have the same outcome, or pass
`--reason` when no evidence exists. `taskfactory requeue` returns an unclaimed
failed task to `tasks/ready` (default) or `tasks/inbox` and commits only that
move; it refuses a failed task that has a Claim block, flagging it for a human
decision, unless `--discard-claim` removes the block and records it in the
commit message. It renames its evidence file aside to
`.taskfactory/evidence/<ID>.attempts-<N>.jsonl` to reset the attempt counter,
and removes the task's old worktree and branch when they are clean and merged
into main; otherwise it refuses and changes nothing.

`taskfactory work` launches the chosen local worker adapter in a claimed task's
worktree. Adapter and model are always explicit; `--haiku-model` is also
required for `claude`. It writes only its log under `.taskfactory/logs/<ID>/`,
and never commits, verifies, integrates or fails a task. A worker exit of 0 is
a claim: run `taskfactory verify <ID>` next.

`taskfactory validate` with no arguments checks the task files in `inbox` and
`ready`. Pass task files or folders to check only those, for example
`taskfactory validate tasks/archive`. Use `taskfactory validate tasks` to check
the whole tree: every state except the archive. Archived tasks are history, so
the whole-tree form does not check their contracts. It still reads them for task
IDs and dependency resolution, and reports an archived file whose ID cannot be
read as an archive read error.

The exact interface will evolve while TaskFactory dogfoods itself.

## Commit signing

TaskFactory commits, including `taskfactory promote` and the archive commit
made by `taskfactory integrate`, run a plain `git commit`. They follow your Git
signing configuration: commits are signed when you have configured signing and
unsigned otherwise. TaskFactory never passes `--no-gpg-sign` or any other
signing override. Automated workers run in an environment where signing is
configured to work or is explicitly disabled by the environment owner, not by
TaskFactory.

## Install

`bin/build` embeds the output of `git describe --dirty`, so untagged commits
build as `<tag>-<N>-g<hash>` (`dev-<hash>` with no tags, `dev` outside a Git
checkout).

The default output is `dist/taskfactory`; `bin/build [output-path]` overrides
it. A default build (or any output inside `dist/`) first empties `dist/`, so
stale release artifacts never mix with fresh ones; an output path elsewhere
removes nothing. `bin/install` puts the binary on PATH.

From a checkout, `bin/install` builds `taskfactory` with the same embedded
version as `bin/build` and writes it to the first of PREFIX, GOBIN, or the
GOPATH bin directory that applies. Set PREFIX to choose a directory. The script
takes no arguments and prints a hint when the directory is not on `PATH`.

It also installs `tf`, a relative symlink to `taskfactory` in the same
directory, so `tf --version` equals `taskfactory --version`. The symlink is
visible to scripts, non-interactive shells and the prompts that
`taskfactory work` builds. Set `TASKFACTORY_NO_ALIAS=1` to skip it. The script
never overwrites an existing `tf` unless it is already a symlink to
`taskfactory` in that directory. If another file, directory or symlink named
`tf` exists, such as Terraform, it installs `taskfactory`, warns on stderr with
the path and still exits 0. If `command -v tf` then resolves to a different
file because an earlier `PATH` entry shadows it, it warns with the shadowing
path. Program output keeps saying `taskfactory` whatever name is invoked.

A shell alias (`alias tf=taskfactory`) is only a fallback for people who cannot
create the symlink, because it is invisible to scripts and non-interactive
shells. `go install` of a remote path is not possible with the current module
name; `bin/install` is the global install. A plain
`go install ./cmd/taskfactory` from a checkout installs only `taskfactory`,
without the alias. Windows is out of scope because of the symlink.

## Shell completion

`taskfactory completion bash|zsh` prints a tab-completion script for both
`taskfactory` and `tf`. It completes commands, flags, flag values and task IDs.

For bash, add this to `~/.bashrc`:

```sh
source <(taskfactory completion bash)
```

For zsh, add this to `~/.zshrc`, after `compdef` is available
(`autoload -Uz compinit && compinit`):

```sh
source <(taskfactory completion zsh)
```

Alternatively, save the zsh script as `_taskfactory` in a directory on `fpath`:
`taskfactory completion zsh > ~/.zfunc/_taskfactory`.

Task IDs are read from the task directories of the repository you are in, and
nothing is offered outside a repository. `bin/install` prints this hint but
never edits your dotfiles.

## Releases

No release has been cut and `bin/release` does not exist yet. The planned
release policy is in [docs/release.md](docs/release.md).

## Agent skill

`skills/taskfactory/SKILL.md` is a portable agent skill covering planner,
orchestrator and worker guidance for the commands that exist today. To use it in
another project, copy the directory into the project's skills location:

```sh
mkdir -p .claude/skills
cp -R /path/to/taskfactory/skills/taskfactory .claude/skills/
```

The skill is also embedded in the binary, so the version of the binary decides
the skill text, with no network or release archive:

```sh
taskfactory skill install                      # write the skill
taskfactory skill status                       # present, missing or stale
taskfactory skill install --global --force     # user-wide, replacing a copy
```

By default the skill goes to `.claude/skills/taskfactory` when `.claude/`
exists in the project root and `.agents/skills/taskfactory` otherwise.
`--target claude`, `--target agents` or `--target dir:<path>` choose the
directory, and `--global` uses `$HOME` instead of the project root. Install
refuses to overwrite an existing directory unless `--force`. It writes
`SKILL.md` and the stamp file `.taskfactory-skill` and nothing else. Status
exits 0 when present and 1 when missing or stale.

Other harnesses may use an equivalent skills directory. The skill lists the
commands that exist, and marks release as not implemented.
`bin/skill.test.sh` checks the frontmatter that harnesses
read.

---

# Why Go?

TaskFactory is mostly:

```text id="ifz97q"
filesystem
+ Git
+ process execution
+ task state
+ CLI
```

Go gives us:

- a small codebase;
- simple maintenance;
- strong standard library;
- excellent built-in testing;
- straightforward filesystem/process handling;
- easy cross-platform compilation;
- a single executable;
- no runtime installation requirement;
- easy future public distribution.

The project follows a **standard-library-first** policy and aims for zero or
near-zero runtime dependencies.

TOML parsing may deliberately be the primary exception.

---

# TaskFactory dogfoods TaskFactory

TaskFactory itself uses:

```text id="pq8u29"
tasks/
├── inbox/
├── ready/
├── active/
├── failed/
└── archive/
```

We want to discover weaknesses in the task protocol by actually building
TaskFactory with TaskFactory's own concepts.

This means the implementation should not arrive as one giant change.

Instead:

```text id="zqcnlk"
idea
  ↓
task
  ↓
success criteria
  ↓
implementation
  ↓
verification
  ↓
integration
  ↓
archive
```

---

# Example projects

TaskFactory examples should eventually live in a separate repository:

```text id="4zkmbk"
~/src/taskfactory-examples
```

Examples should be intentionally tiny.

Their purpose is to teach and test TaskFactory, not demonstrate application
architecture.

For example:

```text id="hj0ip7"
go-simple/
├── calculator.go
└── calculator_test.go
```

with tasks such as:

```text id="8f4q8i"
add subtraction
add multiplication
add division
```

Later examples can deliberately demonstrate:

- four parallel workers;
- dependency chains;
- integration conflicts;
- worker failure;
- stop-the-line;
- remediation.

---

# The design principle

TaskFactory should remain boring.

When choosing between two implementations, prefer the one that is:

1. smaller;
2. simpler;
3. easier to explain;
4. easier to test;
5. easier for an agent to modify;
6. deterministic;
7. based on the Go standard library;
8. dependent on fewer external packages;
9. easier to maintain;
10. easier to distribute.

Add complexity only when actual use demonstrates the need.

---

# In one picture

```text id="fdu1ka"
                           TASKFACTORY

                         🧠 PLANNER
                              │
                      refine / decompose
                              │
                              ▼
     ┌──────────┐       ┌──────────┐
     │ 📥 inbox │──────►│ ✅ ready │
     └──────────┘       └────┬─────┘
                             │ claim
                ┌────────────┼────────────┐
                │            │            │
                ▼            ▼            ▼
              🔧 W1        🔧 W2        🔧 W3       🔧 W4
                │            │            │           │
             worktree      worktree     worktree    worktree
                │            │            │           │
                ▼            ▼            ▼           ▼
              implement → verify → diagnose → repair
                │            │            │           │
                └────────────┴─────┬──────┴───────────┘
                                   │
                              proof packages
                                   │
                                   ▼
                         ┌───────────────────┐
                         │ integration queue │
                         └─────────┬─────────┘
                                   │
                                   ▼
                             🚦 INTEGRATOR
                                   │
                            rebase latest main
                                   │
                                verify
                                   │
                         git merge --ff-only
                                   │
                       ┌───────────┴───────────┐
                       │                       │
                     PASS                    FAIL
                       │                       │
                       ▼                       ▼
                 📦 archive/              💥 failed/
                       │                       │
                       │                retry / escalate
                       │                re-plan / repair
                       │
                       ▼
                   GREEN MAIN

                   RED MAIN?
                       │
                       ▼
                🛑 STOP THE LINE
                       │
                 remediation task
                       │
                       ▼
                   GREEN MAIN
                       │
                       ▼
               resume integration
```

**Plan intelligently. Specify precisely. Work in parallel. Verify locally.
Integrate carefully.**

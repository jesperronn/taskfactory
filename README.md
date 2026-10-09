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

This walks one small task through the whole loop with a local worker. Run it
in a Git repository with at least one commit on `main`. Every step is a command
that exists today; `requeue` is not one of them.

1. Install. From a TaskFactory checkout run `bin/install`, which builds
   `taskfactory` and prints a hint if its directory is not on `PATH`.
2. Init. In your project run `taskfactory init`. It writes
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
3. Plan. Write `tasks/inbox/TF-001-<slug>.md` with a `# TF-001: title`
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
8. Integrate. `taskfactory integrate TF-001` rebases the branch onto main,
   verifies again, fast-forwards main and archives the task.
9. Check main. `taskfactory check-main` reruns main's verification. After
   a successful loop `git status --short` is empty.

If the worker cannot finish (a stall, a refusal, a failed check that it
cannot repair), run `taskfactory fail TF-001 --outcome BLOCKED --reason "..."`
(or `FAILED` after a failed verify attempt). It moves the task to
`tasks/failed/` and commits only that move. There is no `requeue`: to retry,
edit the task by hand into `tasks/ready/` and claim it again.

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

taskfactory init
taskfactory status
taskfactory validate
taskfactory claim
taskfactory verify
taskfactory integrate
taskfactory check-main
taskfactory promote <ID>
taskfactory fail <ID> --outcome <FAILED|BLOCKED>
taskfactory work <ID> --adapter <omp|pi|claude> --model <id>
taskfactory <command> --help
```

`taskfactory fail` moves a claimed active task to `tasks/failed` and commits
only that move. Its last attempt evidence must have the same outcome, or pass
`--reason` when no evidence exists. `taskfactory requeue` does not exist yet.

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

From a checkout, `bin/install` builds `taskfactory` with the same embedded
version as `bin/build` and writes it to the first of PREFIX, GOBIN, or the
GOPATH bin directory that applies. Set PREFIX to choose a directory. The script
takes no arguments and prints a hint when the directory is not on `PATH`.

## Agent skill

`skills/taskfactory/SKILL.md` is a portable agent skill covering planner,
orchestrator and worker guidance for the commands that exist today. To use it in
another project, copy the directory into the project's skills location:

```sh
mkdir -p .claude/skills
cp -R /path/to/taskfactory/skills/taskfactory .claude/skills/
```

Other harnesses may use an equivalent skills directory. The skill lists the
commands that exist, and marks `requeue` and release as not implemented.
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

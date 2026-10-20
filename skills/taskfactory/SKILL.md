---
name: taskfactory
description:
  Plan, orchestrate and work TaskFactory tasks in a Git repository. Use when a
  project uses tasks/ directories (inbox, ready, active, failed, archive), when
  asked to write or validate task contracts, to claim a ready task, or to verify
  or integrate a claimed task with the taskfactory CLI.
---

# TaskFactory

TaskFactory coordinates coding agents through explicit task contracts. Git holds
code, task files on disk hold lifecycle state, `.taskfactory/config.toml` holds
project policy, and the `taskfactory` CLI performs deterministic transitions.

If the project has `docs/protocol-v1.md` and `docs/task-format-v1.md`, they are
authoritative. This skill is self-contained and works without them.

`tf` works anywhere `taskfactory` does (`bin/install` installs both); the full
name is still preferred in docs and generated prompts.

## Commands that exist today

| Command                                       | Purpose                                                                                         |
| --------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `taskfactory init [path]`                     | Create `.taskfactory/config.toml` and the `tasks/` directories; path is any dir in the project. |
| `taskfactory status`                          | Show task counts per state.                                                                     |
| `taskfactory validate [path...]`              | Validate task files; with no path, the inbox and ready task files.                              |
| `taskfactory claim <ID> --owner <name>`       | Move a ready task to active, write its Claim block, create worktree.                            |
| `taskfactory verify <ID>`                     | Run the task's checks in its worktree and append attempt evidence.                              |
| `taskfactory integrate <ID>`                  | Rebase, verify, fast-forward main and archive an accepted task.                                 |
| `taskfactory check-main`                      | Rerun main's verification after integration stopped it.                                         |
| `taskfactory promote <ID>`                    | Move an inbox task to ready and commit only that move.                                          |
| `taskfactory fail <ID> --outcome <O>`         | Move a claimed active task to failed and commit only that move.                                 |
| `taskfactory requeue <ID> [--to <state>]`     | Requeue a failed task; removes its old worktree and branch if safe.                             |
| `taskfactory work <ID> --adapter A --model M` | Launch a local worker in a claimed task's worktree; writes only its log.                        |

Any subcommand not in this table is **not implemented**. Flags that exist are
the ones in a command's `--help` and in this skill, such as `requeue
--discard-claim` below. Do not invent others.

## Not yet implemented

- **Release** (TF-069 to TF-077): `bin/release <version>` exists as a dry run
  (gates, advice, packaging; `--tag` creates a local annotated tag) but the
  release workflow and publishing do not exist yet. The policy is in
  `docs/release.md`. `bin/build` and `bin/package` exist.

## Setup in a new project

1. Use a Git repository with at least one commit. Claim records `HEAD` as the
   base commit.
2. Run `taskfactory init [path]` (the path may be any directory inside the project).
3. Edit `[verification]` in `.taskfactory/config.toml`. `init` writes Go
   defaults (`go test ./...`); replace them with the project's own commands
   for `worker` and `integration`.
4. Commit `.taskfactory/config.toml`, the `tasks/` directories and each ready
   task before claiming. `init leaves .taskfactory/config.toml untracked`, and
   it prints nothing about that. Claim creates task branches from
   `refs/heads/main`, so an uncommitted contract is missing from the worker's
   checkout. Claim does not verify that the config is tracked (not verified).
   Integration does require it tracked in HEAD and unchanged.
5. After claims, `.taskfactory/` operational files and `tasks/active/` show as
   untracked or changed. This is expected; do not commit them by hand. Worker
   evidence in `.taskfactory/evidence/` and integration evidence in
   `.taskfactory/integration-evidence/` are untracked files, not Git commits.
   `work` logs in `.taskfactory/logs/` are untracked too; `integrate` allows
   them and never stages them.

## Ready task contract

A task file is `tasks/<state>/<ID>-<slug>.md`, where `<ID>` is `TF-NNN` and the
slug is lowercase letters, digits and single hyphens. A ready task must have
exactly these headings, in this order, and no `## Claim` block:

```markdown
# TF-001: Short title

## Goal

One or more sentences describing the outcome.

## Dependencies

None

## Scope

What is in scope.

## Constraints

What must not change.

## Success criteria

### C1: Single-line criterion text

Check: go test ./...

### C2: Another criterion

Check: test -f hello.txt

## Verification

Run the checks from the repository root and report each exit code.
```

Rules:

- The first line is `# <ID>: <title>`; the ID must equal the filename ID.
- Dependencies are `None` or `- TF-NNN` lines. A dependency is resolved only
  when that task is in `tasks/archive/`.
- Each `### CN:` heading (C1, C2, ... with no gaps) is followed by exactly one
  `Check:` line holding a single-line command.
- Goal, scope and constraints are non-empty prose.
- Inbox files only need the `# <ID>: <title>` heading.

## Planner

- Refine inbox items into ready contracts using the template above.
- Run `taskfactory promote <ID>` to move the single inbox file for that ID to
  `tasks/ready/`. It validates the file against the ready contract, validates
  the inbox, ready, active and failed task files, and commits only the two task
  paths. A refusal or failure restores the file to `tasks/inbox/` and changes
  nothing else.
- Promotion refuses when the index already has staged paths; unstage them
  first.
- Splitting rule: when part of a task is delivered and the rest is still
  wanted, do not shrink the original and leave it open. Write a new inbox task
  with exactly the remainder, then close the original: append a `Closed`
  section listing each delivered item with its deliverer and a line
  `Remainder: TF-NNN`, and `git mv` it to `tasks/archive/`. Use the same rule to
  split a large task. See `docs/task-format-v1.md`.
- TaskFactory commits follow the user's Git signing configuration. Do not add
  `--no-gpg-sign` or any signing override to TaskFactory commands or to
  commands you run for the user.

## Orchestrator

- Assign ready tasks to workers, keeping no more than `workers.max_parallel`
  (default 4) active.
- Give each worker one task ID. Workers claim with `taskfactory claim`; the
  claim is atomic, so a second claim of the same task fails.
- Read evidence in `.taskfactory/evidence/<ID>.jsonl` (append-only) before
  deciding on retry, escalation or integration.
- Integrate one candidate at a time with `taskfactory integrate <ID>`. Do not
  integrate a task whose latest attempt is not `PASS`.
- A stalled or failed worker is recorded with `taskfactory fail <ID> --outcome
  BLOCKED` (or `FAILED`); `work` prints that command as a suggestion and never
  runs it. To retry, a planner or orchestrator runs `taskfactory requeue <ID>`.
  A failed task that still has a Claim block is refused as a human decision:
  rerun with `--discard-claim` to remove the Claim block (it is recorded in the
  commit message). The old evidence is renamed aside and the attempt counter
  restarts at 1. `requeue` also removes the old worktree and branch, but only
  when the worktree is clean and the branch has no commits that are not on
  main; otherwise it refuses, changes nothing and prints the git commands for
  the human to run.
- If main fails its checks, stop integration and create a remediation task
  (stop-the-line; see `docs/protocol-v1.md` when present).

## Worker

1. Validate the contract: `taskfactory validate tasks/ready/<ID>-<slug>.md`.
2. Claim: `taskfactory claim <ID> --owner <name>`. The command prints only a
   confirmation. Read the branch and `Worktree:` path from the Claim block in
   `tasks/active/<ID>-<slug>.md` (normally `.taskfactory/worktrees/<ID>`), then
   change into that worktree and work only there, on the claimed branch.
3. Implement the task. Do not change success criteria, checks, or tests to make
   them pass.
4. Verify: `taskfactory verify <ID>`. Read the output; diagnose and fix any
   failed check, then verify again.
5. If the project provides them, also run its wrappers (for TaskFactory itself,
   `bin/test` and `bin/lint`) from the repository root and report each exit
   code. Never run `bin/lint --autofix` on unrelated files.
6. Report evidence: task ID, changed files, result commit, each check's command
   and exit code. A failed or blocked result must include the failing command
   and output. Do not report success without that output.

If the contract is impossible or contradictory, report it instead of rewriting
the contract.

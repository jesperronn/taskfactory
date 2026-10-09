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

## Commands that exist today

| Command                                       | Purpose                                                                    |
| --------------------------------------------- | -------------------------------------------------------------------------- |
| `taskfactory init`                            | Create `.taskfactory/config.toml` and the five `tasks/` state directories. |
| `taskfactory status`                          | Show task counts per state.                                                |
| `taskfactory validate [path...]`              | Validate task files; with no path, the inbox and ready task files.         |
| `taskfactory claim <ID> --owner <name>`       | Move a ready task to active, write its Claim block, create worktree.       |
| `taskfactory verify <ID>`                     | Run the task's checks in its worktree and append attempt evidence.         |
| `taskfactory integrate <ID>`                  | Rebase, verify, fast-forward main and archive an accepted task.            |
| `taskfactory check-main`                      | Rerun main's verification after integration stopped it.                    |
| `taskfactory promote <ID>`                    | Move an inbox task to ready and commit only that move.                     |
| `taskfactory fail <ID> --outcome <O>`         | Move a claimed active task to failed; requeue does not exist yet.          |
| `taskfactory work <ID> --adapter A --model M` | Launch a local worker in a claimed task's worktree; writes only its log.   |

Anything not in this table is **not implemented**. Do not invent other
subcommands or flags. Run `taskfactory <command> --help` for flags.

## Not yet implemented

- **Lifecycle fail and requeue** (TF-033): no CLI command moves a task to
  `tasks/failed/` or back to `tasks/ready/`.
- **Worker launch**: adapters for OMP, Pi and Claude Code exist as Go packages,
  but no CLI command starts them. Start workers manually.
- **Release** (TF-032): there is no release script yet. `bin/build` exists.

## Setup in a new project

1. Use a Git repository with at least one commit. Claim records `HEAD` as the
   base commit.
2. Run `taskfactory init`.
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

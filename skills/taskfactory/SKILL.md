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

Read `docs/protocol-v1.md`, `docs/task-format-v1.md` and
`docs/worker-instructions.md` in the project before acting. They are
authoritative; this skill summarizes them.

## Commands that exist today

| Command                                 | Purpose                                                                    |
| --------------------------------------- | -------------------------------------------------------------------------- |
| `taskfactory init`                      | Create `.taskfactory/config.toml` and the five `tasks/` state directories. |
| `taskfactory status`                    | Load config and check the state directories exist.                         |
| `taskfactory validate [task-file]`      | Validate one task file, or the whole task tree when no file is given.      |
| `taskfactory claim <ID> --owner <name>` | Atomically move a ready task to active and write its Claim block.          |
| `taskfactory verify <ID>`               | Run the task's checks in its worktree and append attempt evidence.         |
| `taskfactory integrate <ID>`            | Rebase, verify, fast-forward main and archive an accepted task.            |

Anything not in this table is **not implemented**. Do not invent other
subcommands or flags.

## Not yet implemented

- **Lifecycle commands** (TF-033): there is no CLI command to promote inbox to
  ready, or to move a task to failed or archive by hand. Until then, a planner
  moves the file manually and then runs `taskfactory validate`.
- **Worker adapters** (TF-024 to TF-026): `docs/local-workers-v1.md` describes a
  planned launch contract for OMP, Pi and Claude Code. No adapter exists, so
  workers are started manually.
- **Build and release** (TF-031, TF-032): no `bin/build` or release script yet.

## Planner

- Refine inbox items. Inbox files only need the `# <ID>: <title>` heading; they
  are not executable contracts.
- Promote a task to ready only when it has a goal, scope, constraints,
  dependencies, `### CN:` success criteria each followed by one `Check:` line,
  and a `## Verification` section, in the order required by
  `docs/task-format-v1.md`.
- Dependencies are `- TF-NNN` entries (or `None`). A dependency is resolved only
  when its task is in `tasks/archive/`.
- Run `taskfactory validate tasks/ready/<ID>-<slug>.md` before promoting, then
  `taskfactory validate` for the whole tree.

## Orchestrator

- Assign ready tasks to workers, keeping no more than `workers.max_parallel`
  (default 4) active.
- Give each worker one task ID. Workers claim with `taskfactory claim`; the
  claim is atomic, so a second claim of the same task must fail.
- Read evidence in `.taskfactory/evidence/<ID>.jsonl` (append-only) before
  deciding on retry, escalation or integration.
- Integrate one candidate at a time with `taskfactory integrate <ID>`. Do not
  integrate a task whose latest attempt is not `PASS`.
- If main fails its checks, stop integration and create a remediation task
  (stop-the-line; see `docs/protocol-v1.md`).

## Worker

1. Validate the contract: `taskfactory validate tasks/ready/<ID>-<slug>.md`.
2. Claim: `taskfactory claim <ID> --owner <name>`. Work only in the claimed
   worktree, on the claimed branch.
3. Implement the task. Do not change success criteria, checks, or tests to make
   them pass.
4. Verify: `taskfactory verify <ID>`. Read the output; diagnose and fix any
   failed check, then verify again.
5. Also run the project wrappers from the repository root, reporting each exit
   code: `bin/test` and `bin/lint` (in this project, `bin/lint` must not be run
   with `--autofix` on unrelated files).
6. Report evidence: task ID, changed files, result commit, each check's command
   and exit code. A failed or blocked result must include the failing command
   and output. Do not report success without that output.

If the contract is impossible or contradictory, report it instead of rewriting
the contract.

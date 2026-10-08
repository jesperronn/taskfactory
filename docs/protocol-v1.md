# TaskFactory protocol v1

## Purpose and authority

TaskFactory coordinates coding agents in a Git repository. Git is authoritative for code, task files on disk for lifecycle state, `.taskfactory/config.toml` for project policy, and recorded command results for verification evidence. Agents plan and implement; deterministic CLI operations enforce transitions. No provider-specific service is required.

## Roles

- **Planner:** refines inbox work, splits independent tasks, defines dependencies, scope, success criteria, and checks. Only promotes executable tasks.
- **Orchestrator:** assigns ready tasks, enforces the worker limit, handles failures and remediation, and controls integration state.
- **Worker:** owns one claimed task in an isolated branch/worktree, implements it, verifies it, diagnoses failures, and repairs its own work within its effort budget.
- **Integrator:** serializes integration, rebases and verifies candidates, advances main with `git merge --ff-only`, records evidence, and archives accepted tasks. It does not normally fix feature code.

## Task states

`tasks/inbox/` holds incomplete proposals. `tasks/ready/` holds tasks with a unique ID, goal, scope, constraints, success criteria, verification, declared and resolved dependencies, and enough isolation for execution without project-level planning. `tasks/active/` holds claimed tasks with exactly one owner and records the worker, branch, worktree, base commit, and start time. `tasks/failed/` holds failed or blocked work and its evidence. `tasks/archive/` holds only successfully integrated tasks. A worker's success report alone never archives a task.

A task has a stable ID and human-readable contract. The initial task files use Markdown headings for the contract; the eventual machine-readable representation must be settled before CLI task validation is implemented. Success criteria are fixed before work starts and must not be weakened to obtain a pass. Prefer tests, compilation, type checks, lint, builds, schema/filesystem/Git assertions, and structured comparisons over agent judgment.

## Claim and worker loop

A claim must atomically move one ready task to active so only one claimant succeeds. It must reject unresolved dependencies and excess concurrency. Parallel workers use separate Git branches and worktrees, with a default maximum of four active workers; they do not share mutable directories.

For each task: claim, inspect, implement, verify, diagnose, repair, and repeat until every required criterion passes or the configured effort budget is exhausted. Workers retain failed attempts and must not skip checks, remove failing criteria, or change unrelated code without justification. Outcomes are `PASS`, `FAILED`, or `BLOCKED`. Blocked means an external or specification issue prevents reasonable progress; no separate `blocked/` directory is required. The orchestrator chooses retry, replanning, prerequisite work, escalation, or abandonment.

A passing worker supplies task ID, branch, base and result commits, changed files, verification commands and exit codes, attempt count, and relevant retry history. A failed task retains enough information for another worker to continue.

## Integration and main health

Only one candidate enters the integration critical section at a time:

1. Acquire the integration lock and update main.
2. Rebase the candidate onto current main.
3. Run integration verification on the rebased candidate.
4. Advance main using `git merge --ff-only`.
5. Run any configured main verification, record evidence, and archive the task.
6. Release the lock.

If main changes before the fast-forward, rebase again and rerun verification. On integration failure, record the task and candidate commits, main commit, failing command, exit code, relevant output, and prior worker results. The integrator returns the candidate for remediation rather than editing feature code.

If main is known broken, stop ordinary integration immediately. Existing workers may continue in their worktrees. Prioritize a repair task; resume ordinary integration only after main passes its required checks. A failure of post-merge main verification must leave the pipeline stopped and retain evidence; it must never be silently archived as success.

## Version 1 boundaries

The filesystem remains the task-state store. Provider/model selection and escalation policy belong to the orchestrator; v1 does not require a model-selection engine. No daemon, database, server, remote queue, or generic workflow engine is part of this protocol.

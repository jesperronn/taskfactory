# TaskFactory protocol v1

## Purpose and authority

TaskFactory coordinates coding agents in a Git repository. Git is authoritative
for code, task files on disk for lifecycle state, `.taskfactory/config.toml` for
project policy, and recorded command results for verification evidence. Agents
plan and implement; deterministic CLI operations enforce transitions. No
provider-specific service is required.

## Roles

- **Planner:** refines inbox work, splits independent tasks, defines
  dependencies, scope, success criteria, and checks. Only promotes executable
  tasks.
- **Orchestrator:** assigns ready tasks, enforces the worker limit, handles
  failures and remediation, and controls integration state.
- **Worker:** owns one claimed task in an isolated branch/worktree, implements
  it, verifies it, diagnoses failures, and repairs its own work within its
  effort budget.
- **Integrator:** serializes integration, rebases and verifies candidates,
  advances main with `git merge --ff-only`, records evidence, and archives
  accepted tasks. It does not normally fix feature code.

## Task states

`tasks/inbox/` holds incomplete proposals. `tasks/ready/` holds tasks with a
unique ID, goal, scope, constraints, success criteria, verification, declared
and resolved dependencies, and enough isolation for execution without
project-level planning. `tasks/active/` holds claimed tasks with exactly one
owner and records the worker, branch, worktree, base commit, and start time.
`tasks/failed/` holds failed or blocked work and its evidence. `tasks/archive/`
holds only successfully integrated tasks. A worker's success report alone never
archives a task.

A task has a stable ID and human-readable contract. The initial task files use
Markdown headings for the contract; the eventual machine-readable representation
must be settled before CLI task validation is implemented. Success criteria are
fixed before work starts and must not be weakened to obtain a pass. Prefer
tests, compilation, type checks, lint, builds, schema/filesystem/Git assertions,
and structured comparisons over agent judgment.

## Claim and worker loop

A claim must atomically move one ready task to active so only one claimant
succeeds. It must reject unresolved dependencies and excess concurrency.
Parallel workers use separate Git branches and worktrees, with a default maximum
of four active workers; they do not share mutable directories.

For each task: claim, inspect, implement, verify, diagnose, repair, and repeat
until every required criterion passes or the configured effort budget is
exhausted. Workers retain failed attempts and must not skip checks, remove
failing criteria, or change unrelated code without justification. Outcomes are
`PASS`, `FAILED`, or `BLOCKED`. Blocked means an external or specification issue
prevents reasonable progress; no separate `blocked/` directory is required. The
orchestrator chooses retry, replanning, prerequisite work, escalation, or
abandonment.

A passing worker supplies task ID, branch, base and result commits, changed
files, verification commands and exit codes, attempt count, and relevant retry
history. A failed task retains enough information for another worker to
continue.

### Requeue

`taskfactory requeue <ID> [--to ready|inbox]` returns a failed task to
`tasks/ready/` or `tasks/inbox/`. A failed task that still has a Claim block is
never requeued automatically; it is flagged for a human decision. Requeue
resets the worker attempt counter by renaming
`.taskfactory/evidence/<ID>.jsonl` to
`.taskfactory/evidence/<ID>.attempts-<N>.jsonl`, where N is its highest attempt
number. The rename never overwrites and the old bytes are never rewritten, so
the rule against rewriting evidence still holds. The next attempt is 1.

### Evidence files

Evidence files are untracked. Worker attempts append to
`.taskfactory/evidence/<ID>.jsonl` and integration attempts append to
`.taskfactory/integration-evidence/<ID>.jsonl`. Neither is a Git commit. Verify
does not stage them, and integration stages only the task paths it archives, so
they are untracked. `taskfactory init` writes the project `.gitignore`, or
appends to it, with the runtime paths `.taskfactory/claim.lock`,
`.taskfactory/integration.lock`, `.taskfactory/evidence/`,
`.taskfactory/integration-evidence/`, `.taskfactory/logs/` and
`.taskfactory/worktrees/`, so `git status --short` is empty after a successful
loop. It never ignores `.taskfactory/config.toml`. The runtime inventory reads
the filesystem, not Git status, so ignored paths are still inventoried.
`.taskfactory/config.toml` is written
by init and is not committed by init; it must be tracked in HEAD before
integration, and claim does not verify that it is tracked.

## Integration and main health

Integration is a local operation on the repository's `refs/heads/main`; it does
not fetch, push, contact a remote, or update a remote-tracking ref. It requires
that local `main` exists and is checked out in the repository worktree. The
repository worktree may contain ready-to-active lifecycle changes from
concurrent claims: each must be exactly a ready task file moved to active with
the Claim block appended. It may also contain these TaskFactory operational
paths: `.taskfactory/config.toml` (only when tracked and clean),
`.taskfactory/claim.lock`, `.taskfactory/integration.lock`,
`.taskfactory/evidence/<ID>.jsonl`, `.taskfactory/evidence/<ID>.lock`,
`.taskfactory/integration-evidence/<ID>.jsonl`, the work log directory
`.taskfactory/logs/` with any content (written by `taskfactory work`, never
tracked or staged; `.taskfactory/logsx` and `.taskfactory/logs-old` are not
allowed), `.taskfactory/integration-stop.json`, its atomic-write sibling
`.taskfactory/integration-stop.json.tmp`, and registered worker worktrees
exactly at the configured worktree root plus one task-ID component. `<ID>` must
match `TF-[0-9]{3}`. Claim's `tasks/active/.claim-*.tmp` files and
`.taskfactory/.claim-backup-*.tmp` transaction files are permitted only while
they are transient claim paths. A worktree path is permitted only when Git lists
it as a registered worktree and its task ID matches a retained Claim in
`tasks/active/`, `tasks/failed/`, or `tasks/archive/`. No other tracked or
untracked changes are permitted; in particular, a general `.taskfactory/` or
worktree-root allowance must not hide unrelated files. Claim does not commit
lifecycle state. A parent directory may appear in Git status instead of its
operational children; allow it only when every descendant is an allowed
operational path. Inventory these directory contents even when Git ignore rules
hide them; ignore rules do not grant a blanket runtime allowance. The
integration candidate is the task's claimed worktree. `.taskfactory/config.toml`
must be tracked in HEAD and match both the index and worktree; recheck it before
merge and before lifecycle staging. Any config edit invalidates the check, and
the config is never staged by integration. Only TaskFactory integrations
coordinate through `.taskfactory/integration.lock`; an external Git process does
not honor this lock.

An active task is eligible only when all of these facts hold at the start of an
attempt:

- Exactly one active task file matches the requested ID, its Claim metadata is
  complete, its claimed branch is checked out in the claimed registered Git
  worktree for this repository.
- The complete, schema-valid JSONL file at `.taskfactory/evidence/<ID>.jsonl`
  has consecutive attempt numbers, and its last record has `outcome` `PASS`. Its
  task ID and branch match the active claim. A later `FAILED` or `BLOCKED`
  record supersedes an earlier PASS; malformed or incomplete evidence is not
  skipped.
- The record's `base_commit` matches the Claim `Base commit`; its non-empty
  `result_commit` is a full commit ID; and the candidate worktree HEAD equals
  that result commit. The result is a descendant of its recorded base.
- The candidate worktree has no staged, unstaged, or untracked changes. Thus the
  verified commit is exactly the commit proposed for integration.

The integrator acquires the integration lock before reading the stopped state,
active task, evidence, or main ref, and holds it through evidence append and any
archive transition. Claims use only `.taskfactory/claim.lock`; they never
acquire the integration lock, and integration never acquires the claim lock.
Therefore the locks have no nested acquisition order. A concurrent claim may
capture the old main commit while integration is running; its worker must later
rebase and verify against current main before integration. The claim lock still
serializes claim state changes with other claims.

While holding the integration lock, the integrator performs this sequence:

1. Reject ordinary integration if `.taskfactory/integration-stop.json` exists.
   TF-012 owns creation and rejection of this stop record; TF-013 later adds
   `check-main` and clears it only after successful verification of current
   main. The record is one UTF-8 JSON object with exactly these keys:
   - `main_commit`: full lowercase hexadecimal Git commit ID, 40 or 64
     characters.
   - `command`: exact configured `verification.main` command that failed.
   - `exit_code`: integer process exit code, or null when the command could not
     start.
   - `output`: combined stdout and stderr, or empty when none.
   - `error`: process-start error text, or empty otherwise.

   Example:

   ```json
   {
     "main_commit": "0123456789abcdef0123456789abcdef01234567",
     "command": "go test ./...",
     "exit_code": 1,
     "output": "FAIL example.test/package",
     "error": ""
   }
   ```

   TF-012 writes this file atomically through the sibling temporary path after a
   post-merge main-check failure when `integration.stop_on_main_failure` is
   true. It never clears or overwrites an existing stop record. If the stop file
   or its temporary sibling exists, integration fails closed before changing
   main or task state. For a valid record, the CLI reports its main commit and
   failed command; for a malformed record or leftover temp file, it identifies
   the path for manual inspection. In any case it appends a `BLOCKED`
   integration attempt with stage `eligibility` and directs the operator to
   repair main. TF-012 does not offer a recovery command or delete either file.
   Then check candidate eligibility and record the current full
   `refs/heads/main` commit as `main_before`.

2. Rebase the claimed branch onto that commit. A conflict or failed rebase ends
   the attempt; do not run integration checks or move main. Record the failure
   and return the task for worker remediation.
3. Run every `verification.integration` command, in configured order, at the
   candidate worktree root. A command start error or non-zero exit stops the
   sequence, records the failure, and leaves main and task state unchanged. The
   exact candidate HEAD after rebase is the verified commit.
4. Immediately before merging, read `refs/heads/main` again. If it differs from
   the rebase base, rebase onto the new main commit and rerun all integration
   checks. Repeat until the ref is unchanged at the pre-merge check. Then run
   `git merge --ff-only <verified-commit>` in the main worktree. If the config
   is missing, untracked, or differs from HEAD before merge, record a failure
   and stop without moving main. Any merge error is a failure; never create a
   merge commit or reset main to simulate rollback.
5. If `verification.main` is configured, run every command at the repository
   root after the fast-forward. If one fails, leave this task active. When
   `integration.stop_on_main_failure` is true, write
   `.taskfactory/integration-stop.json.tmp`, then rename it to
   `.taskfactory/integration-stop.json` before appending attempt evidence, so
   later integrations remain stopped even if evidence append fails. If the stop
   record cannot be written, report that main advanced and that stop state could
   not be persisted; still append integration failure evidence when possible,
   and do not claim the pipeline is stopped. TF-013 later implements
   `check-main` recovery. When the setting is false, retain the failure evidence
   and active task but do not create a persistent stop record. Main has already
   advanced; do not claim rollback.
6. After all required checks pass, atomically move only this task file from
   active to archive. Before staging, recheck config cleanliness; if it changed
   after main advanced, move the task back to active, record an archive-stage
   failure, and leave main advanced. Otherwise commit the lifecycle transition
   separately on local main. Claims are uncommitted ready-to-active working-tree
   transitions. When the active file is untracked, stage only its tracked ready
   deletion and archive addition; when the active file is tracked, stage only
   its active deletion and archive addition. Scope every Git add to those exact
   paths; never stage unrelated task or user files. Append the PASS record only
   after that commit succeeds. If the task move or its commit fails, append an
   archive-stage failure record with main left at the verified commit; do not
   report the task archived. If appending the final PASS record fails after the
   archive commit, report that evidence failure and preserve both the main
   commit and archive state; never rewrite older evidence.
7. Release the integration lock on every exit.

The main-ref recheck detects movement caused by another process before the
fast-forward decision. The lock prevents another TaskFactory integration from
moving main in this interval; unrelated Git commands can still race it and are
outside the v1 coordination guarantee. A failed rebase, integration check, or
fast-forward leaves the task active and does not change main. A failed
post-merge main check is the exception: main remains advanced and the task
remains active; when `integration.stop_on_main_failure` is true, TF-013's
persistent stop record governs recovery. Integration attempts are recorded
separately from worker verification in the append-only
`.taskfactory/integration-evidence/<ID>.jsonl` file. Each attempt appends
exactly one record, including failures; it never edits or replaces worker
evidence.

Each integration JSONL record has exactly these keys and types:

- `task_id`: string task ID.
- `attempt`: positive integer, starting at 1 and increasing for every appended
  integration attempt for this task.
- `recorded_at`: RFC 3339 UTC timestamp string.
- `outcome`: `PASS`, `FAILED`, or `BLOCKED`.
- `stage`: one of `eligibility`, `rebase`, `integration_check`, `merge`,
  `main_check`, or `archive`.
- `branch`: claimed branch string, or empty when eligibility did not establish a
  claim.
- `worker_result_commit`: full commit ID from latest worker PASS evidence, or
  empty when eligibility did not establish one. Non-empty commit IDs are full
  lowercase hexadecimal IDs of 40 or 64 characters.
- `main_before`: full main commit ID observed before integration, or empty when
  unavailable; use the same format for non-empty IDs.
- `verified_commit`: full candidate commit ID whose integration checks passed,
  or empty if none; use the same format for non-empty IDs.
- `main_after`: full main commit ID observed after the attempt's last main
  operation, or empty if unavailable; use the same format for non-empty IDs.
- `command`: exact configured command that failed, or empty when the stage has
  no command (including Git operations).
- `exit_code`: integer process exit code, or null if a command could not start
  or failure was not a command exit.
- `output`: combined stdout and stderr for the failing command or Git operation;
  empty if none.
- `error`: process-start or operation error text; empty when none.
- `note`: required string, possibly empty, for concise additional context.

`PASS` is recorded only after all configured integration and main checks pass,
the fast-forward is observed, and the lifecycle archive commit succeeds. A
rebase conflict, nonzero check, rejected fast-forward, or task archive failure
is `FAILED`; inability to start a check is `BLOCKED`. If a post-merge main check
fails, `stage` is `main_check`, `main_after` is the advanced commit, and the
active task is not archived. If eligibility fails before the candidate or main
can be identified, empty commit fields are used and the error is recorded. If
the evidence file cannot be appended, the operation fails without pretending
evidence exists; it does not rewrite older lines. Representative independent
records:

```json
{"task_id":"TF-123","attempt":1,"recorded_at":"2026-10-08T12:00:00Z","outcome":"PASS","stage":"archive","branch":"task/TF-123","worker_result_commit":"1123456789abcdef0123456789abcdef01234567","main_before":"0123456789abcdef0123456789abcdef01234567","verified_commit":"2123456789abcdef0123456789abcdef01234567","main_after":"3123456789abcdef0123456789abcdef01234567","command":"","exit_code":null,"output":"","error":"","note":"Integration checks passed; lifecycle archive commit followed the feature commit."}
{"task_id":"TF-123","attempt":1,"recorded_at":"2026-10-08T12:05:00Z","outcome":"FAILED","stage":"main_check","branch":"task/TF-123","worker_result_commit":"1123456789abcdef0123456789abcdef01234567","main_before":"0123456789abcdef0123456789abcdef01234567","verified_commit":"2123456789abcdef0123456789abcdef01234567","main_after":"2123456789abcdef0123456789abcdef01234567","command":"go test ./...","exit_code":1,"output":"FAIL example.test/package","error":"","note":"Fast-forward already occurred; TF-013 stop state required; task remains active."}
```

The records above are independent examples, each representing attempt 1 in its
own task evidence file. In a real file attempt numbers are consecutive. A task
that was already archived cannot be integrated again.

When the configured stop policy marks main broken, stop ordinary integration
immediately. Existing workers may continue in their worktrees. Prioritize a
repair task; resume ordinary integration only after TF-013's `check-main` passes
on the current main commit and clears `.taskfactory/integration-stop.json`. A
post-merge main failure retains its failure record and stop state; moving main
alone does not clear it.

## Version 1 boundaries

The filesystem remains the task-state store. Provider/model selection and
escalation policy belong to the orchestrator; v1 does not require a
model-selection engine. No daemon, database, server, remote queue, or generic
workflow engine is part of this protocol.

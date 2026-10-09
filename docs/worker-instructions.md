# Worker verification instructions

A worker owns its task's implement, verify, diagnose, and repair loop. Read the
task contract before editing. Keep the branch and worktree isolated, preserve
unrelated changes, and do not weaken success criteria or tests to obtain a pass.

Run the task-specific verification commands, then run `bin/test` and `bin/lint`
from the repository root before returning work. Record each command, exit code,
and relevant output. If either wrapper is missing or unavailable, report that as
a blocker instead of treating it as a pass. After a repair, rerun the affected
check and both wrappers. A passing report includes the task ID, changed files,
result commit, and verification evidence. A failed or blocked report retains the
failing command and enough context to continue.

Before claiming work, validate the ready task with
`go run ./cmd/taskfactory validate tasks/ready/<ID>-<slug>.md`. After editing a
task contract or changing its lifecycle state, run
`go run ./cmd/taskfactory validate tasks` to check the whole tree: inbox, ready,
active and failed. The archive is not contract-checked by this form; it is read
for task IDs and dependency resolution only, and an archived file whose ID
cannot be read is reported as an archive read error. Name `tasks/archive` to
check archived contracts. The single-file form still checks tree-wide ID
uniqueness and dependency references, while reporting only diagnostics for the
selected task. `bin/test` runs the whole-tree form automatically.

## Evidence and operational files

Worker verification evidence is appended to
`.taskfactory/evidence/<ID>.jsonl`. Integration attempts are appended to
`.taskfactory/integration-evidence/<ID>.jsonl`. Both are untracked files under
`.taskfactory/`, not Git commits. `verify` does not stage or commit them, and
`integrate` stages only the task paths it archives. `.gitignore` does not hide
`.taskfactory/`, so `git status` lists these files as untracked. Do not commit
them by hand. After integration, the dry run in
`docs/experiments/e2e-dry-run.md` observed `.taskfactory/evidence/` and
`.taskfactory/integration-evidence/` as untracked; that run was not repeated
for this change.

TaskFactory commits follow the user's Git signing configuration. TaskFactory
runs a plain `git commit` and never overrides signing. Automated workers run in
an environment where signing is configured to work or is explicitly disabled by
the environment owner, not by TaskFactory.

`taskfactory init` writes `.taskfactory/config.toml` and does not commit it, so
the file is untracked until someone commits it. Commit it, with the `tasks/`
contracts, before the first claim. Claim reads the config from the project root
and creates the task branch from `refs/heads/main`, so an uncommitted contract
is missing from the worker's checkout. Claim does not check that the config is
tracked; that refusal is not verified. Integration refuses unless
`.taskfactory/config.toml` is tracked in HEAD and unchanged.

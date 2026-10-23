# Worker verification instructions

A worker owns its task's implement, verify, diagnose, and repair loop. Read the
task contract before editing. Keep the branch and worktree isolated. Preserve
unrelated changes. Do not weaken success criteria or tests to obtain a pass.

Run the task-specific verification commands. Then run `bin/test` and `bin/lint`
from the repository root before returning work. Record each command, exit code,
and relevant output. If either wrapper is missing or unavailable, report that
as a blocker. Do not treat it as a pass.

After a repair, rerun the affected check and both wrappers.

A passing report includes the task ID, changed files, result commit, and
verification evidence. A failed or blocked report keeps the failing command. It
also keeps enough context to continue.

## Task file checks

Before claiming work, run `bin/lint tasks/ready/<ID>-<slug>.md` from the
repository root. It runs the Markdown lint. Then it runs
`go run ./cmd/taskfactory validate` on that file. It prints each failure with
the file name and the check that failed.

Run it again after editing the task file. Its exit code is 0 only when both
halves pass.

The single-file form, `go run ./cmd/taskfactory validate
tasks/ready/<ID>-<slug>.md`, is still available. It runs the validate half
alone. Files under `tasks/archive` are not validated by `bin/lint`.

After editing a task contract or changing its lifecycle state, run
`go run ./cmd/taskfactory validate tasks`. This checks the whole tree: inbox,
ready, active and failed. The archive is not contract-checked by this form. It
is read for task IDs and dependency resolution only. An archived file whose ID
cannot be read is reported as an archive read error.

Name `tasks/archive` to check archived contracts. The single-file form still
checks tree-wide ID uniqueness and dependency references. It reports only
diagnostics for the selected task. `bin/test` runs the whole-tree form
automatically.

The task folders are `tasks/` by default. The `tasks_dir` key in
`.taskfactory/config.toml` can name another directory, for example
`.taskfactory/tasks`. Read that directory wherever this document says `tasks/`.
`bin/lint` and `bin/test` do this for you.

## Evidence and operational files

Two evidence files are appended to, one per task ID:

- Worker verification evidence: `.taskfactory/evidence/<ID>.jsonl`.
- Integration attempts: `.taskfactory/integration-evidence/<ID>.jsonl`.

Both are untracked files under `.taskfactory/`. They are not Git commits.

`verify` does not stage or commit them. `integrate` stages only the task paths
it archives.

`taskfactory work` writes its logs under `.taskfactory/logs/`. `integrate`
allows that directory. It never stages it and needs no manual move.

`taskfactory init` writes a `.gitignore`. It hides these runtime paths under
`.taskfactory/`: the two lock files, `evidence/`, `integration-evidence/`,
`logs/` and `worktrees/`. A project that committed it has an empty
`git status --short` after integrate.

Older projects without those lines list the files as untracked. Do not commit
them by hand. The dry run in `docs/experiments/e2e-dry-run.md` predates the
`.gitignore`. It observed them as untracked.

TaskFactory commits follow the user's Git signing configuration. TaskFactory
runs a plain `git commit` and never overrides signing. Automated workers run
where signing is configured to work. Or the environment owner has disabled it
explicitly. TaskFactory does not decide this.

`taskfactory init` writes `.taskfactory/config.toml` and the `.gitignore`. It
does not commit them, so they are untracked until someone commits them. Commit
both, with the `tasks/` contracts, before the first claim.

Claim reads the config from the project root. It creates the task branch from
`refs/heads/main`, or from the branch `git.base_branch` names. An uncommitted
contract is therefore missing from the worker's checkout.

Claim does not check that the config is tracked. That refusal is not
verified. Integration refuses unless `.taskfactory/config.toml` is tracked in
HEAD and unchanged.

# TF-057: Document the driven loop and run it once end to end

## Goal

Document the full TaskFactory loop now that `work` exists, and prove it by
driving one small task through every command in a scratch repository.

## Dependencies

- TF-047
- TF-048
- TF-056

## Scope

Rewrite the README quick start as: install, init, plan, promote, claim, work,
verify, integrate, check-main, with the `fail` and `requeue` branch. Update
`skills/taskfactory/SKILL.md` and `docs/worker-instructions.md` only where they
disagree with the commands. Run one real small task in a scratch repository
through a local worker with `taskfactory work`. Record the commands, exit codes
and integration commit in `docs/experiments/e2e-local-worker.md`. File
remediation tasks in `tasks/inbox/` for any friction. This replaces the
worker-dispatch part of TF-035; TF-020 (CI run) stays separate.

## Constraints

Record in the experiment record how signing was configured in the scratch
repository. Keep Markdown lines under 81 columns.

## Success criteria

### C1: The README lists work between claim and verify

Check: grep -q "taskfactory work" README.md

### C2: The experiment record exists and names the integration commit

Check: grep -qi "integration commit" docs/experiments/e2e-local-worker.md

### C3: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each C-check, `bin/test` and `bin/lint`, and report each exit code. Report
the scratch repository steps with exit codes, or the blocker if no local model
was available.

**Notes.** Open questions: is the quick start followed verbatim in a scratch repo without
manual fixes (the TF-035 criterion) a hard gate? Which local model and task
size are used? The 2026-10 trials show only slices of about 20 seconds finish
reliably on a local model.

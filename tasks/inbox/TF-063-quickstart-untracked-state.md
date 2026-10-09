# TF-063: Quick start leaves main dirty with untracked .taskfactory files

## Goal

After `integrate`, a project that followed the README has a clean
`git status --short`, or the README says what to add to `.gitignore`.

## Scope

`init` (or the README) must cover the untracked `.taskfactory/claim.lock`,
`evidence/`, `integration-evidence/`, `integration.lock`, `logs/` and
`worktrees/`. The README only mentions the log directory.

## Notes

Found by the verbatim quick start rerun. After a fully successful loop
`git status --short` listed six `.taskfactory/` entries.

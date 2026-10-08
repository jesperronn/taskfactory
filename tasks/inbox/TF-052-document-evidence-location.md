# TF-052: Document where verification evidence lives

## Goal

The docs state that worker and integration evidence are untracked files under
.taskfactory/, not Git commits, and what git status shows after integration.

## Scope

docs/worker-instructions.md and skills/taskfactory/SKILL.md. Also say that
init leaves config untracked and that it should be committed before claim.

## Notes

Found in docs/experiments/e2e-dry-run.md (D3, D4). After integrate, git status
listed .taskfactory/evidence/, .taskfactory/integration-evidence/ and the
lock files. Expected evidence commits did not exist.

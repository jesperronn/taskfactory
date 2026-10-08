# TF-050: Update SKILL.md for the promote command

## Goal

The TaskFactory skill describes the promote command the binary now has,
instead of saying promotion is not implemented.

## Scope

skills/taskfactory/SKILL.md: the command table, the "Not yet implemented"
list and the Planner section. Check README.md for the same drift.

## Notes

Found in docs/experiments/e2e-dry-run.md (D1). `taskfactory promote TF-001`
moved the inbox file to ready and committed it (exit 0). The skill still
says a planner moves the file by hand.

# TF-097: Planner drafting workflow in the skill

## Goal

An agent using the TaskFactory skill can turn an idea into a good inbox task,
with runnable checks.

## Dependencies

- TF-096
- TF-095

## Scope

- Extend the Planner section of `skills/taskfactory/SKILL.md` with a drafting
  workflow: ask what done means, inspect the repository, scaffold with
  `taskfactory new`, write falsifiable `Check:` commands, run each check on a
  clean tree to see it fail first, then validate.
- Add guidance for projects whose tests do not run in a worktree (Docker,
  missing virtualenv): pick checks that run there, or set `[verification]`
  accordingly.

## Constraints

Keep the skill self-contained and keep it listing only commands that exist.
`bin/skill.test.sh` must still pass. Installation is TF-095, not this task.

## Success criteria

### C1: The skill documents the drafting workflow

Check: grep -q "falsifiable" skills/taskfactory/SKILL.md

### C2: The skill still passes its frontmatter test

Check: bin/skill.test.sh

### C3: The whole suite and lint still pass

Check: bin/test && bin/lint

## Verification

Run the three checks from the repository root and report each exit code.

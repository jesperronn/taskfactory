# TF-034: Portable TaskFactory skill for other projects

## Goal

Let any project adopt TaskFactory through an installable agent skill.

## Dependencies

- TF-004
- TF-031

## Scope

No SKILL.md exists today. Write a skill (planner, orchestrator and worker
guidance drawn from protocol-v1 and worker-instructions) plus an install path:
`taskfactory init` scaffolds the task dirs and config, and the skill is copyable
into `.claude/skills/` and equivalent locations for other harnesses. Verify by
adopting it in a scratch repository.

## Success criteria

- A SKILL.md with valid frontmatter exists and is referenced from the README.
- In a fresh scratch repo, `taskfactory init` plus the skill is enough to
  create, validate and claim a task.

## Delivery note

Delivered in `skills/taskfactory/SKILL.md`, with a README section explaining how
to copy it into `.claude/skills/`. The skill documents only the implemented
commands (`init`, `status`, `validate`, `claim`, `verify`, `integrate`) and
marks lifecycle commands (TF-033) and worker adapters (TF-024 to TF-026) as not
implemented. Remaining for the criteria: the scratch-repo adoption check, and
whether the skill needs a `taskfactory init` path that copies it. This task
stays in inbox.

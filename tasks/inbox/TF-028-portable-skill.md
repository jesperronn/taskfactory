# TF-028: Portable TaskFactory skill for other projects

## Goal

Let any project adopt TaskFactory through an installable agent skill.

## Dependencies

- TF-004
- TF-025

## Scope

No SKILL.md exists today. Write a skill (planner, orchestrator and worker
guidance drawn from protocol-v1 and worker-instructions) plus an install path:
`taskfactory init` scaffolds the task dirs and config, and the skill is
copyable into `.claude/skills/` and equivalent locations for other harnesses.
Verify by adopting it in a scratch repository.

## Success criteria

- A SKILL.md with valid frontmatter exists and is referenced from the README.
- In a fresh scratch repo, `taskfactory init` plus the skill is enough to create,
  validate and claim a task.

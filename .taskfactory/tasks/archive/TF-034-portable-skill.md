# TF-034: Portable TaskFactory skill for other projects

## Goal

Let any project adopt TaskFactory through an installable agent skill whose
instructions are sufficient to create, validate and claim a task in a fresh
repository.

## Dependencies

- TF-004
- TF-031

## Scope

The skill lives at `skills/taskfactory/SKILL.md`, is referenced from the README
with copy instructions for `.claude/skills/`, and is self-contained. The
adoption run in a scratch repository is recorded in
`docs/experiments/TF-034-skill-adoption.md`, with each command and exit code.
Frontmatter is checked by `bin/skill.test.sh`, which `bin/test` runs.

## Constraints

Document only implemented CLI commands. Mark lifecycle commands (TF-033) and
worker launch as not implemented. Do not change the CLI or task format. Keep
the skill concise.

## Success criteria

### C1: Skill frontmatter has name and description

Check: bin/skill.test.sh

### C2: README links the skill

Check: grep -q 'skills/taskfactory/SKILL.md' README.md

### C3: Adoption run is recorded with exit codes

Check: grep -q 'taskfactory claim TF-001 --owner worker' docs/experiments/TF-034-skill-adoption.md

### C4: Whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each check from the repository root and report its exit code. Run
`bin/test` and `bin/lint` and report their exit codes. Integration is still
required before this task is archived.

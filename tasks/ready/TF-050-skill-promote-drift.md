# TF-050: Update SKILL.md and README.md for the promote command

## Goal

The TaskFactory skill and README describe the promote and check-main commands
the binary implements, instead of saying promotion is not implemented and that
a planner moves files by hand.

## Dependencies

None

## Scope

skills/taskfactory/SKILL.md: the command table, the "Not yet implemented"
list and the Planner section. README.md: the command list in the CLI section.
No Go code changes.

## Constraints

Describe only behavior that is implemented: `promote` moves one inbox task to
tasks/ready only when its complete contract validates and commits only that
move. Do not claim lifecycle fail or requeue exist; worker adapters are Go
packages only. Keep bin/skill.test.sh passing.

## Success criteria

### C1: Skill command table lists promote

Check: grep -q '^| .taskfactory promote' skills/taskfactory/SKILL.md

### C2: Skill command table lists check-main

Check: grep -q '`taskfactory check-main`' skills/taskfactory/SKILL.md

### C3: Skill no longer says promotion is unimplemented

Check: ! grep -q 'no CLI command to promote' skills/taskfactory/SKILL.md

### C4: Planner section describes promote

Check: grep -q 'taskfactory promote' skills/taskfactory/SKILL.md

### C5: README lists promote and check-main

Check: grep -q 'taskfactory promote' README.md

### C6: Skill test still passes

Check: bin/skill.test.sh

## Verification

Run each check from the repository root and report its exit code, then run
bin/test and bin/lint and report their exit codes.

# TF-062: Quick start must state the Dependencies format

## Goal

A newcomer who writes `None.` under Dependencies must learn the exact format
from the README and from `validate`, before `promote` refuses the file.

## Dependencies

None

## Scope

Inbox files stay loosely validated on purpose: do not make `validate` strict
for them. Instead, README quick start step 3 states the Dependencies format
(`None`, or lines `- TF-NNN`, nothing else, no trailing period) and shows one
complete minimal ready-contract task. `validate --help` says inbox files are
checked loosely and that `promote` applies the full ready contract. When one
explicit argument names a single file in tasks/inbox, `validate` prints a
second stdout line with that hint after its unchanged success line. Directory
arguments and no argument print no hint. `promote` keeps its message that names
the violated rule.

## Constraints

Keep the success line text `N task file(s) valid` unchanged: `bin/lint` and its
tests match it. No new rule in `internal/taskvalidate`. Do not change
`promote` behavior.

## Success criteria

### C1: Validate explains inbox files

Check: go test ./cmd/taskfactory -run TestValidateInboxNote -v

### C2: The README states the format and an example

Check: go test ./cmd/taskfactory -run TestReadmeQuickStartDependencies -v

### C3: Promote still names the rule

Check: go test ./cmd/taskfactory -run TestPromoteNamesDependenciesRule -v

## Verification

Run each check from the repository root with -v and report that it shows PASS.
Run `bin/test` and `bin/lint` and report their exit codes.

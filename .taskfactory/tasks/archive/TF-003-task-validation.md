# TF-003: Parse and validate task contracts

## Goal

Implement `taskfactory validate` so humans and agents can check one task file or
the whole task tree before claiming work.

## Dependencies

- TF-001
- TF-004
- TF-006
- TF-019

## Scope

Parse task files according to `docs/task-format-v1.md`. Support
`taskfactory validate <path>` for one task file and `taskfactory validate` for
all task files under the five state directories. Resolve paths relative to the
current project root. Apply the documented rules for inbox, ready, active,
failed, and archive, including legacy archive exceptions. Enforce unique IDs,
filename/heading ID match, criterion/check pairing, and dependency references
where required. Print diagnostics in stable path order, each with file path and
field. Exit 0 only if selected contracts are valid. A single-file check must
inspect the tree for uniqueness and dependencies while reporting errors for the
selected file only.

Initialize and commit TaskFactory's own `.taskfactory/config.toml` using the
documented defaults, then validate its task tree. Once validation passes, run
whole-tree validation from `bin/test` and document single-file and whole-tree
use for task transitions in `AGENTS.md` and `docs/worker-instructions.md`.

## Constraints

Use the existing Go CLI and config loader. Keep parsing in the Go standard
library and do not introduce a separate validation script or new dependency.
Validation is read-only. Preserve current task IDs, states, and historical
archive content.

## Success criteria

### C1: Documented valid and invalid examples behave as specified

Check: go test ./...

### C2: Duplicate IDs, missing dependencies, and state-specific errors have stable diagnostics

Check: go test ./...

### C3: Single-file and whole-tree validation are read-only

Check: go test ./...

### C4: The TaskFactory repository validates with its committed config

Check: go run ./cmd/taskfactory validate

### C5: The standard test wrapper runs validation, and worker instructions require it

Check: bin/test

### C6: Formatting and static checks pass

Check: bin/lint

## Verification

Run focused parser and CLI tests against temporary task trees, then
`go test ./...`, `go vet ./...`, `bin/test`, and `bin/lint`. Confirm selected
task files are byte-for-byte unchanged after validation. Run whole-tree
validation against this repository and report exact exit codes and diagnostics.
Check that `bin/test` fails when a task file is made invalid in an isolated
fixture and that both invocation forms are included in worker instructions.

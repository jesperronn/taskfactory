# TF-008: Report task state

## Goal

Implement `taskfactory status` as a concise filesystem summary.

## Dependencies

- TF-001
- TF-003

## Scope

From the Git project root, load configuration and validate the task tree using
the existing validator. Count valid task files in inbox, ready, active, failed,
and archive. Print exactly five lines in that order with labels `inbox:`,
`ready:`, `active:`, `failed:`, and `archive:` followed by a decimal count.
Ignore empty `.gitkeep` placeholders. Reject a missing state directory or
invalid task tree with a nonzero exit and a useful path-specific error. Do not
mutate task files, read evidence, or query remote Git.

## Constraints

Reuse the validator and CLI project-root behavior. Keep the command read-only
and avoid a new package or dependency unless needed for a clear boundary.

## Success criteria

### C1: Empty and populated task trees produce exact five-line counts

Check: go test ./...

### C2: Repeated runs are byte-identical and do not modify task files

Check: go test ./...

### C3: Missing directories and invalid task files fail with path-specific diagnostics

Check: go test ./...

### C4: Standard repository checks pass

Check: bin/test

## Verification

Use temporary Git repositories and complete project configs. Test exact output
for empty and populated trees, including empty placeholders. Hash task files
before and after repeated runs. Test missing directories and malformed tasks.
Run `bin/test`, `bin/lint`, `go vet ./...`, and whole-tree validation; report
exact exits.

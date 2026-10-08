# TF-008: Report task state

## Goal

Implement `taskfactory status` as a concise filesystem summary.

## Dependencies

TF-001 and TF-003 must be archived before promotion to ready.

## Scope

Count only task files recognized by `docs/task-format-v1.md` in inbox, ready,
active, failed, and archive from the project root. Print the five counts in the
documented state order with stable labels; do not mutate files, parse agent
evidence, or query remote Git. Report missing or malformed task directories
clearly.

## Success criteria

- Counts are correct for empty and populated temporary task trees.
- Repeated runs produce identical output for unchanged input.
- Missing state directories fail nonzero with the path named.
- `go test ./...` and `go vet ./...` pass.

## Verification

Run CLI tests against temporary trees and verify their files are unchanged.

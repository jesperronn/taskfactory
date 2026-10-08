# TF-021: Define worker verification evidence v1

## Goal

Give TF-011 one unambiguous contract for running task checks and configured
worker checks and recording their outcomes.

## Dependencies

- TF-006
- TF-007

## Scope

Reconcile `docs/task-format-v1.md`, `docs/config-v1.md`, and the TF-011 inbox
story. Specify the exact order and stopping behavior for task criterion checks
and `verification.worker` commands, how both groups appear in append-only
evidence, and how `PASS`, `FAILED`, and `BLOCKED` are recorded. Define exact
JSON keys and types for command output and changed files without silently
overloading `note`. Define attempt numbering and safe append behavior when two
verifications target the same task. Include valid PASS, FAILED, and BLOCKED
examples. Update TF-011's scope and criteria so an implementer can test the
contract directly.

## Constraints

Do not implement `verify` or alter existing task lifecycle files. Keep the
schema small, readable, and implementable with the Go standard library. Preserve
the append-only rule for existing evidence lines and the existing task state
model. Do not claim historical attempts exist.

## Success criteria

### C1: Check execution order and stopping rules are explicit across both sources

Check: cat docs/task-format-v1.md docs/config-v1.md

### C2: Evidence examples cover PASS, FAILED, and BLOCKED with exact keys and types

Check: cat docs/task-format-v1.md

### C3: TF-011 names testable behavior for append, concurrency, and command output

Check: cat tasks/inbox/TF-011-worker-evidence.md

### C4: Repository checks and task validation pass

Check: bin/test

## Verification

Manually inspect C1-C3 against the cited files and record any ambiguity before
marking this task complete. Parse all JSON examples with a real JSON parser and
compare their keys and types to the written schema. Run `bin/test`, `bin/lint`,
and the task validator; report exact exits. These automated checks support the
manual contract review; they do not prove its semantics alone.

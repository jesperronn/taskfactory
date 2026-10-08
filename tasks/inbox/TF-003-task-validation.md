# TF-003: Parse and validate task contracts

## Goal

Implement `taskfactory validate` so humans and agents can check one task file or
the whole task tree before claiming work.

## Dependencies

TF-001, TF-006, and TF-019 must be archived before promotion to ready.

## Scope

Parse task files according to `docs/task-format-v1.md`. Support
`taskfactory validate <path>` for one task file and `taskfactory validate` for
all task files under the five state directories. Resolve paths relative to the
current project root. Apply the state-specific rules in
`docs/task-format-v1.md`: inbox files receive identity and common file checks
only; ready files require the complete executable contract and no Claim; active
files require the complete contract and Claim; failed files require the complete
contract and may retain a valid Claim; archive files require the complete
contract, permit a valid Claim to be absent for legacy records, and accept the
documented legacy criteria and Outcome forms. Enforce unique IDs across the tree
and filename/heading ID match in every state. For complete contracts, validate
criterion/check pairing and dependency references; resolve dependencies only for
these contracts. Print diagnostics in stable path order, each with file path and
field; exit 0 only if all selected contracts are valid and nonzero if any error
exists. A single-file check must inspect the tree to resolve uniqueness and
dependencies, while reporting errors for the selected file only. Do not move
tasks or create worktrees. Do not add a separate validation script.

## Success criteria

- The complete valid example in `docs/task-format-v1.md` passes by path and in
  whole-tree mode.
- Every invalid example in that document fails for its specified reason, with a
  diagnostic naming its file and field.
- Duplicate IDs, missing dependency references, and invalid state directory
  names are tested.
- Validation does not modify any task file.
- `go test ./...` and `go vet ./...` pass.

## Verification

Run focused parser tests, CLI tests for both invocation forms against temporary
task trees, then `go test ./...` and `go vet ./...`. Confirm the task files are
byte-for-byte unchanged after validation.

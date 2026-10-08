# TF-006: Define the machine-readable task contract

## Goal

Specify one human-readable task file format that the Go CLI can validate without
ambiguous parsing rules.

## Dependencies

None.

## Scope

Create `docs/task-format-v1.md`. Define the filename/ID relationship, required
and optional fields, dependency syntax, lifecycle metadata, evidence file
location/format, and how success criteria pair with verification commands.
Include one complete ready-task example and invalid examples for missing
criteria, mismatched ID, and malformed dependencies. Choose either Markdown with
a small structured header or pure TOML as permitted by the technical spec;
explain the parsing boundary and dependency impact. Do not implement parsing or
rewrite existing task files in this task.

## Constraints

Follow `docs/protocol-v1.md` and `docs/technical-spec-v1.md`. Preserve the
current five task directories and the existing task contracts. Keep the schema
narrow enough for a small Go implementation.

## Success criteria

- The document defines every field a ready task needs: ID, title, goal,
  dependencies, scope, constraints, success criteria, and verification.
- A worker can identify the exact check for each success criterion from the
  example.
- The document says which lifecycle metadata is added on claim, where
  append-only attempt evidence lives, and which fields are retained on
  pass/failure.
- Invalid examples have explicit rejection reasons; dependency resolution and ID
  matching have unambiguous rules.
- No Go code, new dependency, or lifecycle move is introduced.

## Verification

Read the document against both v1 specs and the existing `TF-001` ready task.
Run `git diff --check` on this task's changes. Report any existing task file
that will need migration as a follow-up; do not silently change it.

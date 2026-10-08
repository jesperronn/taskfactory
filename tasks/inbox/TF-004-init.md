# TF-004: Initialize a project

## Goal

Implement `taskfactory init` in a Git project.

## Dependencies

TF-001, TF-002, and TF-007 must be archived before promotion to ready.

## Scope

Create `.taskfactory/config.toml` matching `docs/config-v1.md` and the five
task-state directories. Resolve the target project from the current working
directory. If the config already exists, return success without rewriting it or
any task file; create any missing empty state directories. If another
initializer created a conflicting config, report it rather than overwrite it. Do
not create sample tasks or release infrastructure.

## Success criteria

- `init` in an empty temporary Git repository creates the documented layout and
  loadable config.
- Repeating `init` does not change the config or destroy task files.
- Running outside a Git repository fails with an actionable error; nested
  working directories resolve to the Git top-level project.
- `go test ./...` and `go vet ./...` pass.

## Verification

Use temporary Git repositories for success, repeat, and failure cases. Run the
CLI from outside TaskFactory's source checkout.

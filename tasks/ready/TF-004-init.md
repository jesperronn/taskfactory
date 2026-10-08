# TF-004: Initialize a project

## Goal

Implement `taskfactory init` in a Git project.

## Dependencies

- TF-001
- TF-002
- TF-007

## Scope

Create `.taskfactory/config.toml` matching `docs/config-v1.md` and the five
task-state directories. Resolve the target project from the current working
directory. If the config already exists, return success without rewriting it or
any task file; create any missing empty state directories. If another
initializer created a conflicting config, report it rather than overwrite it. Do
not create sample tasks or release infrastructure.

## Constraints

Use the existing Go CLI and config loader. Keep initialization safe to rerun and
avoid changing files outside the current Git project.

## Success criteria

### C1: An empty Git project gains a loadable default configuration and five task directories

Check: go test ./...

### C2: Repeating init preserves config bytes and existing task files

Check: go test ./...

### C3: Nested directories use the Git top level and non-Git directories fail clearly

Check: go test ./...

### C4: Repository verification passes

Check: bin/test

## Verification

Use temporary Git repositories for success, repeat, and failure cases. Assert
the created config is loadable through `internal/config.Load`, compare file
contents and modification times on rerun, and run the CLI from outside
TaskFactory's source checkout. Run `bin/test`, `bin/lint`, and `go vet ./...`
and report exact exit codes.

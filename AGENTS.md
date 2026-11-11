# TaskFactory

A Go CLI and file protocol that turns Markdown task contracts into verified,
integrated work by AI workers.

## Definition of done

`bin/workflows` lists every command. Before reporting a code or documentation
task complete, run these from the repository root and report each exit code:

```sh
bin/test
bin/lint
```

Before claiming a task, and after editing one, run
`bin/lint .taskfactory/tasks/ready/<ID>-<slug>.md`. It checks the Markdown style
and the task contract (the same `go run ./cmd/taskfactory validate` step) and
reports each failure with its file name. Before claiming, the single-file
contract check alone is
`go run ./cmd/taskfactory validate .taskfactory/tasks/ready/<ID>-<slug>.md`.
After changing task files or moving a task between states, run `bin/lint` with
no arguments, or `go run ./cmd/taskfactory validate .taskfactory/tasks`, to
validate the whole tree: every state except the archive's contract checks.
Archived tasks are read for IDs and dependencies only, and `bin/lint` does not
validate files under `.taskfactory/tasks/archive`. `bin/test` also runs
whole-tree validation.

Repair failures you introduced and rerun both checks. If a required check cannot
run, report the blocker and the command output; do not claim it passed. Follow
the task's own success criteria and verification commands as well. The full
worker loop is in [worker instructions](docs/worker-instructions.md).

## Conventions

- Conventional commits; one logical change per commit; stage files by path.
- Shell tests sit next to their script as `bin/<name>.test.sh`, use
  `bin/testlib.sh`, and stay hermetic and under about 1s (`bin/test --budget`).
  Run one with `bin/test bin/<name>.test.sh`.
- Use `gsed`, not BSD `sed`, on macOS.
- New `bin/` scripts get a `# <name> — <one line>` header, which feeds
  `bin/workflows`.

## Hard rules

- Never use `bin/lint --autofix` on unrelated files.
- Never run `bin/release`, `bin/release-publish`, tag, push or publish; a human
  does that ([release policy](docs/release.md)).

## Layout

- `bin/`: maintainer commands and their tests; `bin/detail-check` proves a
  simplified Markdown file lost no detail from its original.
- `skills/`: the agent skill, embedded in the binary.
- `dist/`: git-ignored output of `bin/build`.
- `node_modules/`: local tooling (prettier); not tracked.

## Docs

- [Worker instructions](docs/worker-instructions.md), [CI and Dependabot](docs/ci.md)
- [Release policy](docs/release.md), [protocol](docs/protocol-v1.md), [task format](docs/task-format-v1.md)

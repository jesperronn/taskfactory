# TaskFactory worker checks

Before reporting a code or documentation task complete, run these from the
repository root and report each exit code:

```sh
bin/test
bin/lint
```

Before claiming a task, and after editing one, run
`bin/lint tasks/ready/<ID>-<slug>.md`. It checks the Markdown style and the
task contract (the same `go run ./cmd/taskfactory validate` step) and reports
each failure with its file name. Before claiming, the single-file contract
check alone is `go run ./cmd/taskfactory validate tasks/ready/<ID>-<slug>.md`.
After changing task files or moving a task between states, run
`bin/lint` with no arguments, or
`go run ./cmd/taskfactory validate tasks`, to validate the whole tree: every
state except the archive's contract checks. Archived tasks are read for IDs and
dependencies only, and `bin/lint` does not validate files under
`tasks/archive`. `bin/test` also runs whole-tree validation.

Repair failures you introduced and rerun both checks. If a required check cannot
run, report the blocker and the command output; do not claim it passed. Never
use `bin/lint --autofix` on unrelated files. Follow the task's own success
criteria and verification commands as well. See
[worker instructions](docs/worker-instructions.md).

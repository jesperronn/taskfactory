# TaskFactory worker checks

Before reporting a code or documentation task complete, run these from the
repository root and report each exit code:

```sh
bin/test
bin/lint
```

Before claiming a task, validate its ready contract with
`go run ./cmd/taskfactory validate tasks/ready/<ID>-<slug>.md`. After changing
task files or moving a task between states, run
`go run ./cmd/taskfactory validate tasks` to validate the whole tree: every
state except the archive's contract checks. Archived tasks are read for IDs and
dependencies only. `bin/test` also runs whole-tree validation.

Repair failures you introduced and rerun both checks. If a required check cannot
run, report the blocker and the command output; do not claim it passed. Never
use `bin/lint --autofix` on unrelated files. Follow the task's own success
criteria and verification commands as well. See
[worker instructions](docs/worker-instructions.md).

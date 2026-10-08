# TaskFactory worker checks

Before reporting a code or documentation task complete, run these from the
repository root and report each exit code:

```sh
bin/test
bin/lint
```

Repair failures you introduced and rerun both checks. If a required check cannot
run, report the blocker and the command output; do not claim it passed. Never
use `bin/lint --autofix` on unrelated files. Follow the task's own success
criteria and verification commands as well. See
[worker instructions](docs/worker-instructions.md).

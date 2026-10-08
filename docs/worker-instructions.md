# Worker verification instructions

A worker owns its task's implement, verify, diagnose, and repair loop. Read the
task contract before editing. Keep the branch and worktree isolated, preserve
unrelated changes, and do not weaken success criteria or tests to obtain a pass.

Run the task-specific verification commands, then run `bin/test` and `bin/lint`
from the repository root before returning work. Record each command, exit code,
and relevant output. If either wrapper is missing or unavailable, report that as
a blocker instead of treating it as a pass. After a repair, rerun the affected
check and both wrappers. A passing report includes the task ID, changed files,
result commit, and verification evidence. A failed or blocked report retains the
failing command and enough context to continue.

Before claiming work, validate the ready task with
`go run ./cmd/taskfactory validate tasks/ready/<ID>-<slug>.md`. After editing a
task contract or changing its lifecycle state, run
`go run ./cmd/taskfactory validate tasks` to check all five task directories. The
single-file form still checks tree-wide ID uniqueness and dependency references,
while reporting only diagnostics for the selected task. `bin/test` runs the
whole-tree form automatically.

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

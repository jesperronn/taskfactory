# TF-065: Say that worktrees and branches remain after integrate and fail

## Goal

After `integrate` or `fail`, a newcomer knows that the task worktree and its
branch remain, and how to remove them.

## Dependencies

None

## Scope

README.md quick start only: the paragraphs after step 9 and the one that
starts "If the worker cannot finish".

## Constraints

Say only what the experiment showed. Do not change any command.

## Success criteria

### C1: The README names the removal command for worktrees

Check: grep -q 'git worktree remove' README.md

### C2: The README names the removal command for branches

Check: grep -q 'git branch -d' README.md

### C3: The README says how to list what remains

Check: grep -q 'git worktree list' README.md

## Verification

Run each check from the repository root and report its exit code. Run
`bin/test` and `bin/lint` and report their exit codes.

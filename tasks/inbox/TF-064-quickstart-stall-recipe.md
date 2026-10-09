# TF-064: Document how to make a worker stall for the fail branch

## Goal

The README quick start says how a newcomer provokes a stalled worker, so the
fail branch can be followed without guessing the timeout.

## Dependencies

None

## Scope

README.md quick start only: the paragraph after step 9 that covers the fail
branch. Add the stall recipe there.

## Constraints

Do not change the commands or their behavior. Quote the real output of the
built binary, not an invented one.

## Success criteria

### C1: The README gives the one second timeout recipe

Check: grep -q 'timeout 1s' README.md

### C2: The README quotes the suggested fail command

Check: grep -q 'suggestion, not run' README.md

## Verification

Run each check from the repository root and report its exit code. Run
`bin/test` and `bin/lint` and report their exit codes.

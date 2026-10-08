# TF-014: Ensure lint warnings fail the command

## Goal

When Prettier reports Markdown formatting errors, `bin/lint` must return a
nonzero exit status.

## Dependencies

TF-005.

## Scope

Reproduce the reported behavior with a temporary `npx` stub that prints a
Prettier-style warning and exits 1. Check the exit status of `bin/lint` itself.
If it already returns 1, preserve its implementation and add a focused
regression test or verification script for the behavior. If it returns 0, fix
the exit-status propagation and add the regression check. Keep `--autofix`
behavior and Bash/Go placeholders intact.

## Constraints

Do not reformat existing Markdown files. Do not depend on registry access;
`npx prettier` may try to download packages in restricted environments. Preserve
unrelated working-tree changes.

## Success criteria

- A mocked Prettier formatting failure makes `bin/lint` exit nonzero.
- A mocked successful Prettier run makes `bin/lint` exit zero.
- `--autofix` still passes `--write` and propagates the linter exit status.
- The regression check runs locally without network access.

## Verification

Run the regression check, `bash -n bin/lint`, and report whether the original
script was actually faulty. Do not run `--autofix` against repository Markdown.

## Outcome

The original `bin/lint` was already correct: its `exec npx` passes through the
Prettier exit status. Added `bin/lint.test.sh` to check statuses 0 and 1 in both
default and `--autofix` modes, plus exact arguments. The check and Bash syntax
validation pass. The real Prettier command could not be reproduced in this
environment because the npm registry is unreachable.

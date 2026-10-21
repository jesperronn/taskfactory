# TF-015: Verify lint failure with real Prettier

## Goal

Fix the reported case where `bin/lint` prints Prettier formatting warnings but
returns exit code 0.

## Dependencies

- TF-014

## Scope

Reproduce the behavior using the real Prettier executable invoked by `bin/lint`.
Use an isolated temporary directory containing a copy of `bin/lint`, one
deliberately misformatted Markdown file, and one formatted Markdown file. Make
Prettier available there without substituting or mocking `npx` or Prettier.
Capture the output and exit status of `bin/lint` itself, then diagnose and fix
the cause if it exits 0 after reporting formatting errors. Add an automated
regression check that exercises real Prettier. Keep the existing mocked checks
as a separate check of argument and exit-status forwarding.

## Constraints

Do not run `--autofix` against repository files or reformat unrelated Markdown.
Do not change the Markdown lint command's intended formatting rules. Preserve
unrelated working-tree changes. If real Prettier cannot be run, record the
blocker and leave the task unfinished; a mock alone cannot satisfy this task.

## Success criteria

- Running real `bin/lint` against an intentionally misformatted Markdown file
  names that file or reports formatting issues and exits nonzero.
- Running real `bin/lint` against a correctly formatted Markdown file reports no
  formatting issues and exits 0.
- Both results are asserted by an automated regression check using real
  Prettier, with no fake `npx` or fake formatter.
- Existing `bin/lint.test.sh` checks continue to pass, including `--autofix`
  argument handling and exit-status forwarding.

## Verification

Run the real-Prettier regression check and `bin/lint.test.sh`. Record the exact
commands, observed output, exit codes, and Prettier version. Explain the root
cause before marking the task complete. Do not infer the exit code from the
shell prompt or from warning text alone.

## Outcome

Completed in `6214844`. The reported exit-0 behavior did not reproduce with real
Prettier 3.9.9: `bin/lint` exited 1 and named the misformatted file, then exited
0 with no warnings for a formatted file. Its final `exec npx prettier` already
forwards the formatter's exit status, so no production script change was needed.
`bin/lint.real-prettier.test.sh` and `bin/lint.test.sh` both pass. The three
harness experiments and their separate commits are recorded in
`docs/experiments/TF-015-worker-comparison.md`.

# TF-041: Lint every task file together with the validate step

## Goal

Make one command check both the Markdown style and the task contract of every
task file, so a task cannot be claimed, committed or integrated with either kind
of defect.

## Dependencies

- TF-038 must conclude with mdsmith in place of Prettier.
- TF-030 (Go and shell lint with path arguments) is related and should land
  first or be merged with this task.

## Scope

Today `bin/lint` checks Markdown formatting only and `taskfactory validate`
checks the task contract only; `bin/test` runs whole-tree validation. Add a
combined check, for example a `bin/check-task <file>` helper, that runs the
Markdown linter and `taskfactory validate <file>` on one task file and fails if
either fails with both sets of diagnostics shown. Make `bin/lint` with no
arguments also run whole-tree `taskfactory validate` after the Markdown lint,
and make `bin/lint <task-file>` run the single-file validation for task files
under `tasks/`. Update `AGENTS.md` and `docs/worker-instructions.md` so workers
run the combined check before claiming and after editing a task file. Decide
whether `taskfactory validate` itself should call a configured Markdown lint
command; prefer keeping the Go CLI independent of the Markdown tool and doing
the combination in the shell wrapper.

## Constraints

Keep exit codes 0, 1 and 2 unchanged. Do not make the Go CLI depend on mdsmith.
Failures must name the file and which of the two checks failed.

## Notes

Success criteria to write when promoting to ready: a deliberately misformatted
task file fails the combined check on the lint half, a task file with a broken
contract fails on the validate half, and a clean task file passes both. Check
each with the test script style used by `bin/lint.test.sh`.

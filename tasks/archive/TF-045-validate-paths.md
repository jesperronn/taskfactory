# TF-045: Validate takes several files and folders

## Goal

Let `taskfactory validate` check exactly the task files a user names, or the
active backlog by default, and fail if any one of them is invalid.

## Dependencies

- TF-003

## Scope

Extend `validate` only; `verify <ID>` keeps its meaning (it runs a claimed
task's success-criteria checks). New behavior of
`taskfactory validate [path...]`:

1. With no arguments, validate every task file in `tasks/inbox` and
   `tasks/ready`.
2. With one or more arguments, each is a task file or a directory. A directory
   means every task file directly inside it, so `tasks` means all five state
   directories (the whole tree) and `tasks/archive` means only the archive.
   Arguments are resolved from the project root as today and must lie inside
   the tasks directory.
3. Tree-wide checks (ID uniqueness, dependency references) always run over the
   whole tree, but diagnostics are reported only for the selected files, as the
   single-file form does today.
4. Print one diagnostic line per failure, naming the file. Exit 1 if any
   selected file fails, 0 only if all pass, 2 for invalid usage such as a path
   outside the tasks directory or a file that is not a task file.
5. `taskfactory verify` given something that looks like a path (contains a slash
   or ends in `.md`) keeps exiting 2 but adds the hint: "did you mean
   `taskfactory validate <path>`?".
6. Update `validateHelp`, `bin/test` and its tests, `AGENTS.md`,
   `docs/worker-instructions.md`, the README and `docs/technical-spec-v1.md` so
   whole-tree validation is spelled `taskfactory validate tasks` and the
   no-argument form is described as inbox plus ready.

## Constraints

Do not change `verify <ID>` behavior or exit codes. Keep whole-tree validation
available and used by `bin/test`. No new dependencies. Preserve color behavior
from TF-042 and the help conventions from TF-043.

## Success criteria

### C1: No arguments validates inbox and ready only

Check: go test ./cmd/taskfactory -run ValidatePaths

### C2: Several files and folders work and any failure exits 1

Check: go test ./internal/taskvalidate ./cmd/taskfactory

### C3: Whole tree is validated through the tasks folder

Check: go run ./cmd/taskfactory validate tasks

### C4: Repository checks pass

Check: bin/test

## Verification

Run the four checks and `bin/lint`. In a scratch copy, break one ready task and
one archived task: confirm no arguments reports only the ready one, `validate
tasks/archive` reports the archived one, naming both files together exits 1, and
a path outside `tasks` exits 2. Also run `taskfactory verify tasks/inbox/x.md`
and confirm the hint. Report each exit code.

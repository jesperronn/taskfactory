# TF-041: Lint every task file together with the validate step

## Goal

Make `bin/lint` check both the Markdown style and the task contract of a task
file, so a task cannot be claimed, committed or integrated with either kind of
defect.

## Dependencies

- TF-038
- TF-030
- TF-045

## Scope

Today `bin/lint` checks Markdown formatting only, and `taskfactory validate`
checks the task contract only. Change the shell wrapper `bin/lint` in two ways.
With no arguments, run the Markdown lint and then whole-tree
`go run ./cmd/taskfactory validate tasks`. With a path under `tasks/` that ends
in `.md`, run the Markdown lint on that file and then
`go run ./cmd/taskfactory validate <file>`. Both halves run even if the first
fails, and each failure prints the file name and which check failed. Do not
add a separate helper script.

Update `AGENTS.md` and `docs/worker-instructions.md` so that workers run
`bin/lint <task-file>` before claiming a task and after editing one.

Keep the Go CLI independent of mdsmith. Do not change `cmd/` or `internal/`.
Add `bin/lint.task.test.sh` in the style of `bin/lint.test.sh`, with fixtures
built under `mktemp -d`.

## Constraints

Keep exit codes 0, 1 and 2 unchanged. Do not make the Go CLI depend on mdsmith
(go.mod and the Go sources must not mention it). Failures must name the file
and which of the two checks failed. Do not edit anything under
`tasks/archive/`.

## Success criteria

### C1: A misformatted task fails the lint half

Check: bin/lint.task.test.sh lint-fail

### C2: A broken task contract fails the validate half

Check: bin/lint.task.test.sh validate-fail

### C3: A clean task passes both halves

Check: bin/lint.task.test.sh clean

### C4: Exit codes and the Go CLI are unchanged

Check: ! grep -rqi mdsmith cmd internal go.mod && bin/test

### C5: The worker docs name the combined check

Check: grep -q "bin/lint tasks/" AGENTS.md && grep -q "bin/lint tasks/" docs/worker-instructions.md

## Verification

Run each check from the repository root and report its exit code. Run `bin/lint`
and report its exit code. Before and after the TF-039 and TF-040 changes, run
`bin/lint` on one inbox task, record the output and state whether new findings
appear. Paste one failing and one passing combined output with the file name.

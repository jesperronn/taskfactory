# TF-030: Lint Go and shell, accept path arguments

## Goal

Make `bin/lint` meet the global lint contract and cover Go, not only Markdown.

## Dependencies

- TF-005

## Scope

`bin/lint` currently runs only Prettier on `**/*.md` and has TODOs for Bash and
Go. It rejects file arguments with exit 2. Add `go vet ./...` and a `gofmt -l`
check, lint Bash scripts under `bin/` with shellcheck when installed (otherwise
print an explicit skip notice), accept any number of file or directory paths,
and keep `--autofix` (gofmt -w, prettier --write) limited to the given paths.
With no arguments, lint the whole project.

## Constraints

Preserve existing exit codes for usage errors and for the real-Prettier failure
case covered by `bin/lint.real-prettier.test.sh`. Do not reformat unrelated
files and do not weaken existing lint tests.

## Success criteria

### C1: Path arguments lint only those paths

Check: bin/lint docs/worker-instructions.md

### C2: A badly formatted Go file fails lint

Check: bin/lint.test.sh

### C3: Whole-project lint and repository tests pass

Check: bin/test

## Verification

Run `bin/lint.test.sh`, `bin/lint docs/worker-instructions.md`, `bin/lint`, and
`bin/test`, plus `go run ./cmd/taskfactory validate`. Report each exit code.

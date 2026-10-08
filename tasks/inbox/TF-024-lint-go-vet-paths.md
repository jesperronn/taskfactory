# TF-024: Lint Go and shell, accept path arguments

## Goal

Make `bin/lint` meet the global lint contract and cover Go, not only Markdown.

## Dependencies

- TF-005

## Scope

`bin/lint` currently runs only Prettier on `**/*.md` and has TODOs for Bash and
Go. It rejects file arguments (exit 2). Add `go vet ./...` and a `gofmt -l`
check, lint Bash scripts under `bin/` (shellcheck if installed, otherwise report
the skip explicitly), accept any number of file or directory paths, and keep
`--autofix` (gofmt -w, prettier --write) limited to the given paths.

## Success criteria

- `bin/lint` with no arguments runs vet, gofmt, Bash and Prettier checks.
- `bin/lint docs/worker-instructions.md` lints only that file and exits 0.
- A deliberately mis-formatted Go file fails `bin/lint <file>` non-zero.
- `bin/lint.test.sh` covers path arguments and Go failures.

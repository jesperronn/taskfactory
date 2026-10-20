# TF-082: Document why bin/build writes to bin/out

## Goal

Keep `bin/out/taskfactory` as the default `bin/build` output and record why,
so the choice is not reopened by accident. The Go default is
`go build` writing `./taskfactory` in the current directory, and `go install`
writing to GOBIN; neither fits here. The repository root would be cluttered
and `bin/` already holds tracked scripts, which is why a separate ignored
`bin/out/` exists. `dist/` is the goreleaser convention for release archives,
and is kept free for the release packaging of TF-072. `build/` would add a
second top-level output directory for no gain. Developers who want the Go
default pass an explicit path, and `bin/install` covers the GOBIN case.

## Dependencies

None

## Scope

Documentation and test only; no behaviour change. Decisions are final:

1. `README.md` gets a short paragraph in the `bin/build` description naming
`bin/out/taskfactory` as the default, saying `bin/build [output-path]`
overrides it, saying `bin/install` is the way to put the binary on PATH, and
giving the rationale above in two or three sentences.
2. `bin/build.test.sh` gains one scenario `output-default`, which checks that
the default path is `bin/out/taskfactory`, that an explicit path is honored and
that `.gitignore` lists `bin/out/`. Add the scenario in the same
scenario-per-argument style as TF-081 if that task has landed; otherwise add
it as a plain argument-selected block that keeps running with no argument.
3. `docs/release.md` is not changed; release archive layout belongs to TF-072.

## Constraints

Touch only `README.md` and `bin/build.test.sh`. Do not change `bin/build`,
`bin/install`, `.gitignore` or Go code. No new dependencies.

## Success criteria

### C1: The default path and the ignore rule are tested

Check: bin/build.test.sh output-default

### C2: The README names the default output path

Check: grep -q 'bin/out/taskfactory' README.md

### C3: The README mentions the dist directory is left to release packaging

Check: grep -q 'dist/' README.md

### C4: The full test suite passes

Check: bin/test

## Verification

Run `bin/build.test.sh output-default`, `bin/test`, `bin/lint` and
`go run ./cmd/taskfactory validate tasks` from the repository root and report
each exit code.

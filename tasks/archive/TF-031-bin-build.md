# TF-031: Add bin/build

## Goal

Provide one reproducible command that builds the `taskfactory` binary with an
embedded version string.

## Dependencies

- TF-001

## Scope

Add `bin/build` that runs `go build` to produce `bin/out/taskfactory` (ignored
by Git), embedding the version from `git describe --always --dirty` into the
`version` variable so `taskfactory --version` prints it. Support an optional
output path argument. Add a `bin/build.test.sh` test, discovered automatically
by the existing `bin/*.test.sh` loop in `bin/test`. Add a build step to
`.github/workflows/ci.yml` after tests.

## Constraints

Use only the Go standard library and existing toolchain. Keep the default
`version` value `dev` so `taskfactory --version` without ldflags still prints
`taskfactory version dev`. Do not weaken existing tests. Do not commit `bin/out`
artifacts.

## Success criteria

### C1: bin/build creates the binary

Check: bin/build && test -x bin/out/taskfactory

### C2: The built binary prints the embedded version

Check: bin/build && bin/out/taskfactory --version

### C3: Binary output is ignored by Git

Check: git check-ignore -q bin/out/taskfactory

### C4: CI builds the binary

Check: grep -q bin/build .github/workflows/ci.yml

### C5: The bin/build test script passes

Check: bin/build.test.sh

## Verification

Run `bin/build`, `bin/out/taskfactory --version`, `bin/build.test.sh`,
`bin/test`, and `bin/lint` from the repository root, plus
`go run ./cmd/taskfactory validate`. Report each exit code and the printed
version.

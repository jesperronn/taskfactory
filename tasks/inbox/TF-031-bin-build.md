# TF-031: Add bin/build

## Goal

Provide one reproducible command that builds the `taskfactory` binary.

## Dependencies

- TF-001

## Scope

Add `bin/build` producing `bin/out/taskfactory` (gitignored) with `go build`,
embedding a version string from `git describe --always --dirty` that
`taskfactory --version` prints. Support an optional output path. Call it from CI
after tests.

## Success criteria

- `bin/build` exits 0 and creates the binary.
- `bin/out/taskfactory --version` prints the embedded version.
- Binary output is ignored by Git and the CI workflow builds it.

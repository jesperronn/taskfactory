# TF-081: Dev-style version string in bin/build

## Goal

`bin/build` embeds the output of `git describe --dirty`, so a binary built
from untagged commits carries a dev-style version such as `v1.2.0-18-g8fb0a59`
and is never mistaken for a release. Today `git describe --always --dirty`
prints a bare hash such as `4583d46` when there are no tags.

## Dependencies

None

## Scope

Change only the version derivation in `bin/build` and extend
`bin/build.test.sh`. Decisions are final:

1. The version is the output of plain `git describe --dirty`: the tag name
when HEAD is tagged (such as `v1.2.0`), otherwise `<tag>-<N>-g<hash>`, with
`-dirty` appended on a dirty tree. No `--long`, no `--match`, no `--tags`.
2. If `git describe` fails because there are no tags, the version is
`dev-<abbreviated hash>`.
3. Not a Git checkout, git missing, or a repository without commits: `dev`,
the same default as `main.version`.
4. `bin/build` still accepts one optional output path and prints
`built <path> (version <version>)`.
5. `bin/build.test.sh` runs one named scenario per argument and all of them
with no argument, which is what `bin/test` does. The scenarios are `tagged`,
`untagged`, `dirty`, `notag` and `nogit`. An unknown name exits 2. Each
scenario prints a line starting with its name and PASS and builds in its own
temporary Git repository with `commit.gpgsign` set to false in that
repository's own config.
6. The README states the rule in one or two sentences next to the `bin/build`
description.

## Constraints

Touch only `bin/build`, `bin/build.test.sh` and `README.md`. No Go changes; the
`-X main.version` mechanism stays. Tests use temporary directories, no network
and no global Git config changes. Do not create tags in the real repository.

## Success criteria

### C1: HEAD at a tag prints the tag name

Check: bin/build.test.sh tagged

### C2: Commits after a tag print tag-N-ghash

Check: bin/build.test.sh untagged

### C3: A dirty tree appends -dirty

Check: bin/build.test.sh dirty

### C4: With no tags the version is dev-hash

Check: bin/build.test.sh notag

### C5: Outside a Git checkout the version is dev

Check: bin/build.test.sh nogit

### C6: The README documents the rule

Check: grep -q 'git describe' README.md

### C7: The full test suite passes

Check: bin/test

## Verification

Run `bin/build.test.sh` with no argument and with each scenario name,
`bin/test`, `bin/lint` and `go run ./cmd/taskfactory validate tasks` from the
repository root and report each exit code.

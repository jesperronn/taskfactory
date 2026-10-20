# TF-081: Dev-style version string in bin/build

## Goal

`bin/build` prints a plain release version only for a clean checkout of an
official release tag, and a dev-style version such as `v22.1.0-18-g8fb0a59`
everywhere else, so a binary built from untagged commits is never mistaken for
a release. Today `git describe --always --dirty` prints a bare hash such as
`4583d46` when there are no tags.

## Dependencies

None

## Scope

Change only the version derivation in `bin/build` and extend
`bin/build.test.sh`. Decisions are final:

1. Release marker: an annotated tag matching `v[0-9]*`, found by
`git describe --long --match 'v[0-9]*'` without `--tags`, so lightweight tags
(scratch bookmarks) never count. The glob is deliberately loose; the strict
version grammar belongs to `bin/release.lib.sh` from TF-070 and TF-071, which
this task neither uses nor needs. An exact `v1.0.0-rc.1` tag therefore prints
as itself.
2. Exact release: HEAD is exactly the tag and the tree is clean. The version is
the plain tag, such as `v1.2.0`.
3. Otherwise the version is the `--long` form `<tag>-<N>-g<hash>`, such as
`v1.2.0-18-g8fb0a59`. A clean checkout at the tag with a dirty tree gives
`v1.2.0-0-g<hash>-dirty`.
4. Dirty: append `-dirty` to whatever the rule above gives, as `--dirty` does,
for tracked-file changes only.
5. No tag at all: `v0.0.0-<N>-g<hash>` where N is `git rev-list --count HEAD`
and the hash is the abbreviated HEAD hash. Reason: it has the same shape as a
tagged dev version, so one parser handles both, and it sorts below every real
release. `dev-<hash>` is rejected because it breaks that shape.
6. Not a Git checkout, git missing, or a repository without commits: `dev`,
the same default as `main.version`.
7. `bin/build` still accepts one optional output path and prints
`built <path> (version <version>)`.
8. `bin/build.test.sh` runs one named scenario per argument and all of them
with no argument, which is what `bin/test` does. The scenarios are `exact`,
`dev`, `dirty`, `notag`, `lightweight`, `nogit` and `nocommit`. An unknown
name exits 2. Each scenario prints a line starting with its name and PASS and
builds in its own temporary Git repository with `commit.gpgsign` set to false
in that repository's own config.
9. The README states the rule in a short paragraph next to the `bin/build`
description.

## Constraints

Touch only `bin/build`, `bin/build.test.sh` and `README.md`. No Go changes; the
`-X main.version` mechanism stays. Tests use temporary directories, no network
and no global Git config changes. Do not create tags in the real repository.

## Success criteria

### C1: A clean checkout at an annotated tag prints the plain tag

Check: bin/build.test.sh exact

### C2: Commits after a tag print the long dev form

Check: bin/build.test.sh dev

### C3: A dirty tree appends -dirty, including at a tag

Check: bin/build.test.sh dirty

### C4: With no tags the version is v0.0.0-N-gHASH

Check: bin/build.test.sh notag

### C5: A lightweight tag is ignored

Check: bin/build.test.sh lightweight

### C6: Outside a Git checkout the version is dev

Check: bin/build.test.sh nogit

### C7: A repository without commits gives dev

Check: bin/build.test.sh nocommit

### C8: The README documents the rule

Check: grep -q 'v0.0.0-' README.md

### C9: The full test suite passes

Check: bin/test

## Verification

Run `bin/build.test.sh` with no argument and with each scenario name,
`bin/test`, `bin/lint` and `go run ./cmd/taskfactory validate tasks` from the
repository root and report each exit code.

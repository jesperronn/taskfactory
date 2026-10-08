# TF-026: Add bin/release

## Goal

Produce versioned release artifacts for installing TaskFactory in other
projects.

## Dependencies

- TF-025

## Scope

Note: technical-spec-v1 excludes a public release mechanism from the initial
implementation, so this stays in inbox until that is revisited. Design
`bin/release <version>`: require a clean main with passing `bin/test` and
`bin/lint`, tag, and cross-build darwin/linux arm64/amd64 archives with
checksums. Publishing to GitHub requires explicit confirmation; default is a
local dry run.

## Success criteria

- `bin/release --dry-run v0.0.0-test` builds archives and checksums in a temp
  directory and does not tag or push.
- Release is refused on a dirty tree or failing checks.

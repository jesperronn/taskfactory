# TF-072: Cross-build and packaging

## Goal

Build the release artifacts for a version: four binary archives, the skill
archive and one checksum file, in a chosen output directory.

## Dependencies

- TF-069

## Scope

Add `bin/package <version> <output-dir>` (bash). It produces:

1. `taskfactory_<version>_<os>_<arch>.tar.gz` for darwin and linux on arm64
   and amd64. Each holds the `taskfactory` binary and `LICENSE`. Built with
   `CGO_ENABLED=0`, `-trimpath` and `-ldflags "-X main.version=<version>"`.
2. `taskfactory-skill_<version>.tar.gz` holding a `taskfactory/` directory
   with `SKILL.md`. ASSUMPTION: the owner has not answered the skill format
   question; this is the recommended layout and the owner may override it
   (for example zip, or extra files).
3. `SHA256SUMS` covering all five archives, produced with `sha256sum` or
   `shasum -a 256`, whichever exists.

The version argument is validated by the same rule as the library from
TF-070 when it exists, and by a local copy of the rule otherwise; keep one
place to change. The output directory must be empty or absent. The script
never tags, never pushes and never needs the network (the Go module cache
must already hold the dependencies).

## Constraints

Bash only; no Go changes and no new dependencies. Do not claim reproducible
builds. The host-architecture archive is run in the test to check `--version`;
the other archives are only checked for contents.

## Success criteria

### C1: All five archives and the checksum file are produced

Check: bin/package.test.sh archives

### C2: The host binary prints the given version

Check: bin/package.test.sh version

### C3: The skill archive has the documented layout

Check: bin/package.test.sh skill

### C4: The checksum file verifies

Check: bin/package.test.sh checksums

### C5: Bad versions and non-empty output directories are refused

Check: bin/package.test.sh refuse

### C6: The whole test script passes

Check: bin/package.test.sh

## Verification

Run every check from the repository root, then `bin/test` and `bin/lint`, and
report each exit code. Cross-builds need only the Go toolchain.

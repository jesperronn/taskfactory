# TF-079: taskfactory skill install

## Goal

Add `taskfactory skill install`, which downloads the skill archive of a
release, verifies it and installs it for any harness. The skill is not
embedded in the binary.

## Dependencies

- TF-072

## Scope

A new Go package (for example `internal/skillinstall`) plus dispatch and
per-command `--help` in `cmd/taskfactory/main.go`, in the style of the other
commands, with exit codes 0, 1 and 2 and colors through `internal/ui`.
Standard library only (`net/http`, `crypto/sha256`, `archive/tar`,
`compress/gzip`).

1. Source: the GitHub release of the binary's own embedded version (the
   `-X main.version` value), or `--version <v>`. A `dev` or dirty version
   without `--version` is refused with a clear message. It fetches
   `taskfactory-skill_<version>.tar.gz` and `SHA256SUMS`. `--base-url` replaces
   the download base for tests and mirrors. `--from <file.tar.gz>` with
   `--sums <file or url>` installs offline.
2. Verify the SHA-256 of the archive against `SHA256SUMS`. Refuse on a
   mismatch or a missing entry, before extracting anything.
3. Extract safely: only the single `taskfactory/` directory, reject absolute
   paths, `..` components and symlinks or other non-regular entries, enforce
   size limits (entry count, per-file and total bytes).
4. Destination, relative to the project root: `.agents/skills/taskfactory` by
   default. `--target agents` is the same, `--target claude` uses
   `.claude/skills/taskfactory`, `--target dir:<path>` uses that path.
   `--global` uses the user-level directory of the chosen target
   (`~/.agents/skills` or `~/.claude/skills`). `--force` overwrites; without
   it the command refuses and names what exists. It never touches other
   directories.
5. Help and docs say honestly that the checksum comes from the same release,
   so it proves integrity and not publisher identity; the signed annotated
   tag does that.

## Constraints

No new dependencies. Tests use an `httptest` server and temporary directories
only: never the real network and never the real home directory (the home path
comes from `HOME`, set to a temp dir in tests). No signing override anywhere.
Do not bundle or embed the skill in the binary.

## Success criteria

### C1: A verified archive installs into the default directory

Check: go test ./internal/skillinstall -run InstallDefault -v

### C2: Checksum mismatch and a missing entry refuse before extracting

Check: go test ./internal/skillinstall -run ChecksumRefuses -v

### C3: Unsafe archive entries are rejected

Check: go test ./internal/skillinstall -run UnsafeEntries -v

### C4: Targets, global and an existing directory behave as specified

Check: go test ./internal/skillinstall -run TargetsAndForce -v

### C5: Offline install with from and sums works

Check: go test ./internal/skillinstall -run OfflineInstall -v

### C6: The CLI help names the skill install command

Check: go test ./cmd/taskfactory -run SkillInstallHelp -v

### C7: The whole package passes vet and tests

Check: go vet ./... && go test ./...

## Verification

Run every check from the repository root, then `bin/test` and `bin/lint`, and
report each exit code. Only a human can verify the real download: after the
first release exists (TF-076), run `taskfactory skill install` in a scratch
project against the real GitHub release and confirm the files and checksum
check. Workers must not claim that.

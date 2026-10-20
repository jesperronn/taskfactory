# TF-073: bin/release dry run, gates and annotated tag

## Goal

Add `bin/release <version>`: gates, advice, packaging and an annotated tag
behind an explicit flag, with a signing gate. It never pushes in this task.

## Dependencies

- TF-070
- TF-071
- TF-072

## Scope

`bin/release <version> [--tag] [--allow-unsigned]`, default a dry run:

1. Validate the version with the library. Gates: branch is `main`, clean
   tree, the tag does not exist, no upstream divergence when an `origin`
   exists, then `bin/test` and `bin/lint` pass. A test hook lets tests stub
   the two checks through an environment variable naming the commands.
2. Print the last tag, the advised bump and any warnings, then apply the
   check function. The advice never blocks except for a version not greater
   than the last tag.
3. Run `bin/package` into a temporary directory (dry run) and print what a
   real run would do, including the exact push command for a human.
4. With `--tag`: refuse unless `git config --get tag.gpgSign` is true or
   `--allow-unsigned` is given, then create the tag with the command
   `git tag -a <version> -m "Release <version>"`, following the user's
   config. The script never passes `-s`, `-u`, `--no-sign` or any signing
   flag. Nothing is pushed. Without `--tag`, no tag is created.

Update `docs/release.md` and the SKILL.md Release line to match.

## Constraints

Bash only. Automated workers never push or publish and never create a real
tag in the project repository; tests use temporary repositories whose own
config sets `tag.gpgSign` and `commit.gpgsign` as needed. A test greps the
script for the forbidden signing flags. No Go changes.

## Success criteria

### C1: A dry run builds archives and creates no tag

Check: bin/release.test.sh dry-run

### C2: Dirty tree, wrong branch and failing checks are refused

Check: bin/release.test.sh gates

### C3: The tag is annotated with the Release message

Check: bin/release.test.sh tag

### C4: Tagging is refused without tag.gpgSign unless allowed

Check: bin/release.test.sh signing

### C5: The script contains no signing flags

Check: bin/release.test.sh no-signing-flags

### C6: The whole test script passes

Check: bin/release.test.sh

## Verification

Run every check from the repository root, then `bin/test` and `bin/lint`, and
report each exit code. Never run `bin/release --tag` in the real repository.

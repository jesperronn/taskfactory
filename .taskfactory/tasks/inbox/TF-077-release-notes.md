# TF-077: Release notes

## Goal

Decide how release notes are produced. Today the tag message and the GitHub
release body are just `Release <version>`. The format is decided before the
stable `v1.0.0`, using real feedback from `v1.0.0-rc.1`, which ships with the
plain message.

## Dependencies

- TF-076

## Scope

Unrefined: the owner wants to discuss this first, after the rc. The owner
rejected these earlier:

1. Tag message and release body generated from the commits only.
2. A committed `CHANGELOG.md` updated at each release.
3. One file per release under `docs/releases/<version>.md`.

Of the newer options the owner picked "defer". Two alternatives are kept as
remembered options, not decided:

- Hand-written notes passed to the release with `--notes <file>`.
- GitHub auto-generated release notes.

Open questions to bring: where the text lives, who writes it, whether
conventional commits feed it, and how it stays consistent between the tag
message and the GitHub release body. The skill install task (TF-080) is
separate and does not depend on this.

## Constraints

Do not promote or start this task before the owner discussion. Do not publish
release notes or any release without an explicit owner ask. Write a decision
only; the owner picks the format.

## Success criteria

### C1: The chosen release notes format is documented

Check: grep -q 'release notes' docs/release.md

### C2: Docs lint clean

Check: bin/lint docs/release.md

## Verification

Run both checks from the repository root and report each exit code. Report
the open questions that the owner still has to answer.

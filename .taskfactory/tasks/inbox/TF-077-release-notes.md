# TF-077: Release notes

## Goal

Decide and implement how release notes are produced. Today the tag message
and the GitHub release body are just `Release <version>`. The format is
decided before the stable `v1.0.0`, using real feedback from
`v1.0.0-rc.1`. It is deferred past rc.1, which ships with the plain message.

## Dependencies

- TF-076

## Scope

Unrefined: the owner wants to discuss this first, after the rc. Do not
promote or start before that discussion. The owner rejected these earlier:

1. Tag message and release body generated from the commits only.
2. A committed `CHANGELOG.md` updated at each release.
3. One file per release under `docs/releases/<version>.md`.

Of the newer options the owner picked "defer". Two alternatives are kept as
remembered options, not decided:

- Hand-written notes passed to the release with `--notes <file>`.
- GitHub auto-generated release notes.

Open questions to bring: where the text lives, who writes it, whether
conventional commits feed it, and how it stays consistent between the tag
message and the GitHub release body. The skill install tasks (TF-079,
TF-080) are separate additional release tasks and do not depend on this.

## Notes

Unrefined backlog item: not promoted, no success criteria yet. Depends on the
first release so the real flow can be seen before deciding.

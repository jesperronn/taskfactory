# TF-077: Release notes

## Goal

Decide and implement how release notes are produced. Today the tag message
and the GitHub release body are just `Release <version>`.

## Dependencies

- TF-076

## Scope

Unrefined: the owner wants to discuss this first. Do not promote or start
before that discussion. The owner rejected these three options:

1. Tag message and release body generated from the commits only.
2. A committed `CHANGELOG.md` updated at each release.
3. One file per release under `docs/releases/`.

The new proposal needs the owner's conversation. Open questions to bring:
where the text lives, who writes it, whether conventional commits feed it,
and how it stays consistent between the tag and the GitHub release.

## Notes

Unrefined backlog item: not promoted, no success criteria yet. Depends on the
first release so the real flow can be seen before deciding.

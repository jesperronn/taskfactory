# Release policy

This document is the reference for how TaskFactory is released. The release
tasks TF-070 to TF-077 implement it.

## Status

Planned, not implemented. Today only `bin/build` (builds `taskfactory`
with an embedded version) and `bin/install` exist. `bin/release` and the
release workflow do not exist yet. They are planned in these tasks:

- TF-070: version parsing and bump advice
- TF-071: prerelease versions (`-rc.N`)
- TF-072: cross-build and packaging
- TF-073: `bin/release` dry run, gates and annotated tag
- TF-074: GitHub Actions release workflow
- TF-075: `bin/release --publish` local fallback
- TF-076: first release `v1.0.0-rc.1`
- TF-077: release notes (decided before the stable `v1.0.0`)
- TF-078: `taskfactory skill install`
- TF-079: `npx skills add` compatibility and the install paths in the README

## Releases are human-started

Releases are never automatic. Only a human running `bin/release <version>`
starts one, and the default is a dry run. Nothing in the commit history
triggers a release, and no commit type does either.

## Tags and versions

Tags are annotated and named `v<major>.<minor>.<patch>`, optionally followed
by `-rc.<N>`. The first release is `v1.0.0-rc.1`. SemVer precedence applies,
so `v1.0.0-rc.1` sorts before `v1.0.0`. Tags with an `-rc` label are
published as GitHub prereleases. The version parser is one isolated function
so other prerelease labels can be added later.

The tag message is `Release <version>` and the GitHub release body is the
same text. Release notes are deferred past `v1.0.0-rc.1`: the format is
decided before the stable `v1.0.0`, with feedback from the rc (TF-077). Until
then there are no generated notes.

## Signing

The scripts never pass `-s`, `-u` or any signing flag. Signing follows the
user's own git configuration. `bin/release` refuses to create the tag unless
`tag.gpgSign` is true in that configuration, or the human passes
`--allow-unsigned`.

## Bump advice

Conventional commits are used only for the bump advice `bin/release` prints:

- major: `!` after the type or scope, or a `BREAKING CHANGE` or
  `BREAKING-CHANGE` footer
- minor: `feat`
- patch: everything else, that is `fix`, `perf`, and also a range where
  nothing qualifies (only docs, test, ci, build, chore, refactor or style
  commits, or subjects that do not follow the convention)

The advice is monotone: a range never suggests more than its largest
qualifying commit. Subjects that do not follow the convention are listed as
warnings.

The advice never blocks. `bin/release` warns when the given version is lower
than the advice. Only a version not greater than the last release tag is
refused. The first release has no previous tag, so the human names its
version explicitly.

## Artifacts

- `taskfactory_<version>_<os>_<arch>.tar.gz` for darwin and linux on arm64
  and amd64 (four archives)
- `taskfactory-skill_<version>.tar.gz` holding a `taskfactory/` directory
  with `SKILL.md` and nothing specific to one harness (no `.claude`
  paths). It follows the Agent Skills layout, where `SKILL.md` has a name and
  a description.
- `SHA256SUMS`, one file covering all archives

## Publishing

Two paths publish a release, and both are idempotent:

- A GitHub Actions workflow runs on a pushed tag and uses only the built-in
  `GITHUB_TOKEN`. This is the normal path.
- The local fallback `bin/release --publish` is for when the workflow is
  unavailable. It may push the tag.

The race: a tag push from the local path also triggers the workflow, so the
two can run at the same time. Idempotency means whichever runs second finds
the existing release and does not create a duplicate. Prefer the workflow;
use the fallback only when it cannot run.

## Automated workers

Pushing and publishing are human actions. Automated workers never push a
tag or publish a release. Every push or publish path needs an explicit flag
and a human running it.

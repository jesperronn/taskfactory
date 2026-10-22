# Release policy

This document is the reference for how TaskFactory is released. The release
tasks TF-070 to TF-077 implement it.

## Status

Partly implemented. `bin/build`, `bin/install`, `bin/package` and the
`bin/release` dry run with `--tag` exist. The release workflow and
publishing do not exist yet. They are planned in these tasks:

- TF-070: version parsing and bump advice
- TF-071: prerelease versions (`-rc.N`)
- TF-072: cross-build and packaging
- TF-073: `bin/release` dry run, gates and annotated tag
- TF-074: GitHub Actions release workflow
- TF-075: `bin/release --publish` local fallback
- TF-076: first release `v1.0.0-rc.1`
- TF-077: release notes (decided before the stable `v1.0.0`)
- TF-095: `taskfactory skill install`
- TF-080: `npx skills add` compatibility and the install paths in the README

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

## The `bin/release` command

`bin/release <version> [--tag [--publish]] [--allow-unsigned]` is a dry run by default
and changes nothing in the repository. It collects and reports all gate
failures together and exits 1 if any failed (usage errors exit 2). The gates
are: the branch is `main`, the tree is clean (untracked files included),
local `main` equals `origin/main` after a `git fetch origin`, the tag does
not exist, and `bin/test` and `bin/lint` pass. It then prints the last tag
and the bump advice, runs `bin/package` into a temporary directory, and
prints the exact `git push origin <tag>` command for a human. With `--tag` it
also creates the annotated tag locally with
`git tag -a <version> -m "Release <version>"`. It never pushes unless
`--publish` is given (see Publishing).

Tests override parts through environment variables: `RELEASE_TEST_CMD`,
`RELEASE_LINT_CMD`, `RELEASE_PACKAGE_CMD`, `RELEASE_NO_ORIGIN_OK=1` (skip the
fetch and origin gate when there is no origin remote) and `RELEASE_REPO_DIR`.

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
  a description. It remains for `npx skills add` and manual use; the
  `taskfactory skill install` command uses the skill embedded in the binary,
  not this archive.
- `SHA256SUMS`, one file covering all archives

## Publishing

Two paths publish a release, and both are idempotent:

- A GitHub Actions workflow runs on a pushed tag and uses only the built-in
  `GITHUB_TOKEN`. This is the normal path.
- The local fallback `bin/release --tag --publish` is for when the workflow
  is unavailable. It may push the tag.

### Local fallback: `bin/release <version> --tag --publish`

`--publish` requires `--tag` (otherwise exit 2), is never the default and is
the only path that pushes or calls `gh`. `--allow-unsigned` works as for
`--tag`. The order is:

1. The TF-073 gates, including the signing gate.
2. `gh release view <version>`. If a release already exists, or its state
   cannot be read (anything other than "release not found"), the script exits
   1 before creating the tag or pushing anything, so the repository is left
   unchanged.
3. `bin/package` runs once and its output is reused.
4. The annotated tag is created locally, then pushed with
   `git push origin <version>`.
5. `gh release view` is checked again. If the workflow already created the
   release, only missing or changed assets are uploaded
   (`bin/release-publish`, `gh release upload --clobber`) and the script
   prints "release already created by the workflow, assets reconciled".
   Otherwise `bin/release-publish` creates the release and the script prints
   "created by local publish".

If publishing fails after the push, the tag is on origin and the script says
so; the workflow may have published the release, so the human checks and
decides.

The race: the tag push also starts the workflow. The workflow may create the
release between the second check and the publish, so both paths are
idempotent instead of exclusive: whichever runs second finds the existing
release and only reconciles assets. The workflow is the normal path. Use the
local path only when Actions is unavailable, and then do not wait for or rerun
the workflow. Because the tag push starts it anyway, check the Actions run
afterwards. What real GitHub and `gh` do here is unverified until a human
runs it with a real push.

## Automated workers

Pushing and publishing are human actions. Automated workers never push a
tag or publish a release. Every push or publish path needs an explicit flag
and a human running it.

# TF-069: Release policy and docs

## Goal

Revisit the v1 non-goal "public release mechanism" and write down the release
policy that the other release tasks implement, so the code tasks have one
reference.

## Dependencies

None

## Scope

Add `docs/release.md` and update the places that say release is excluded or
not implemented. Document these owner decisions as the policy:

1. Only a human running `bin/release` creates a release tag. No commit type
   triggers a release and nothing releases automatically.
2. Tags are annotated and named `v<major>.<minor>.<patch>`, optionally
   followed by `-rc.<N>`. The first release is `v1.0.0-rc.1`. SemVer
   precedence applies, so `v1.0.0-rc.1` sorts before `v1.0.0`. The version
   parser is one isolated function so other prerelease labels can be added
   later. Tags with an `-rc` label are published as GitHub prereleases.
3. Conventional commits are used only for the bump advice `bin/release`
   prints: major for `!` after the type or scope or a `BREAKING CHANGE` or
   `BREAKING-CHANGE` footer, minor for `feat`, patch for `fix` and `perf`.
   When nothing qualifies the advice is a minor ("always suggest a minor").
   Consequence, stated plainly: a fix-only range suggests patch while a
   docs-only range suggests minor. Non-conforming subjects are
   listed as warnings. The advice never blocks: it warns when the given
   version is lower than the suggestion, and only a version not greater than
   the last release tag is refused. For the first release the human names the
   version.
4. The tag message is `Release <version>`, and the GitHub release body is
   the same message. The format of release notes is deferred to a later
   discussion (see the release notes task).
5. Signing: the scripts never pass `-s`, `-u` or any signing flag. `bin/release`
   refuses to create the tag unless the user's git config has `tag.gpgSign`
   set to true, or the human passes `--allow-unsigned`.
6. Artifacts: `taskfactory_<version>_<os>_<arch>.tar.gz` for darwin and linux
   on arm64 and amd64, plus `taskfactory-skill_<version>.tar.gz` holding a
   `taskfactory/` directory with `SKILL.md` (assumption, owner may override),
   and one `SHA256SUMS` file.
7. Publishing: a GitHub Actions workflow on tag push, and a local fallback
   `bin/release --publish`. State honestly which to use when and the race
   between them (a tag push from the local path also triggers the workflow).
8. Automated workers never push or publish. Every push or publish path needs
   an explicit flag and a human. The default is a dry run.

Update the non-goals sentence and the "Do not build release infrastructure
yet" sentence in `docs/technical-spec-v1.md` to say the release mechanism is
specified in `docs/release.md`. Update the Release line in
`skills/taskfactory/SKILL.md` (it says there is no release script) to point at
the policy and say the script is planned; the line must stay true until the
script lands. Add a README link.

## Constraints

Documentation only. No code and no script changes. Markdown lines stay under
81 columns. Do not edit archived tasks. Describe only what exists or is
clearly marked planned.

## Success criteria

### C1: The policy document exists and names the tag rule

Check: grep -q 'v1.0.0-rc.1' docs/release.md

### C2: The signing rule is documented

Check: grep -q 'tag.gpgSign' docs/release.md

### C3: The spec no longer says release is excluded without a pointer

Check: grep -q 'docs/release.md' docs/technical-spec-v1.md

### C4: The skill still validates and points at the policy

Check: bin/skill.test.sh

### C5: The docs lint clean

Check: bin/lint docs/release.md docs/technical-spec-v1.md README.md

## Verification

Run every check from the repository root, then `bin/test` and `bin/lint`, and
report each exit code. Read `docs/release.md` once more against the list in
Scope and confirm every numbered item is present.

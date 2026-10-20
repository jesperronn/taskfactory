# TF-074: GitHub Actions release workflow

## Goal

Publish a release from a pushed tag in GitHub Actions, idempotently, using
only the built-in token. Add the shared publish script the local fallback will
reuse.

## Dependencies

- TF-071
- TF-072

## Scope

1. `bin/release-publish <version> <dist-dir> [--verify-tag]` (bash, uses `gh`).
   `--verify-tag` checks that the tag is annotated (not lightweight), matches
   `v<major>.<minor>.<patch>[-rc.<N>]` and points at a commit reachable from
   `origin/main`. Any failure exits non-zero and nothing is published.
   Publishing creates the GitHub release when `gh release view` finds none, with
   the title and body
   `Release <version>` and the prerelease flag for `-rc` versions; when a
   release exists it only uploads missing assets and replaces assets whose
   checksum differs (`gh release upload --clobber`), and never edits or
   deletes the release itself. Re-running is safe.
2. `.github/workflows/release.yml` on `push` of tags `v*`, with
   `permissions: contents: write` and no other secrets. Steps: checkout
   with full history and tags, Go 1.25.11 (same as CI), tag validation
   through the `--verify-tag` option, then `bin/test` and `bin/lint` (both
   must pass before anything is built), `bin/package`, then publish, with the
   token
   passed as `GH_TOKEN` from `github.token`. The workflow never runs on pull
   requests.
3. Tests use a fake `gh` on `PATH` that records calls and simulates "release
   exists" and "release missing", and temporary repositories for the tag
   checks.

Document in `docs/release.md` that a tag push triggers this workflow.

## Constraints

Only `GITHUB_TOKEN`. No network in tests. Automated workers never push a tag
or run the workflow. Whether the workflow behaves as intended on GitHub can
only be confirmed by a human pushing a tag (see the first release task); do
not claim it works from a local run. Keep the `actions/checkout` and
`actions/setup-go` majors the same as `ci.yml`.

## Success criteria

### C1: A missing release is created with the prerelease flag for rc

Check: bin/release-publish.test.sh create

### C2: An existing release only gets assets uploaded

Check: bin/release-publish.test.sh idempotent

### C3: A lightweight tag is refused and nothing is published

Check: bin/release-publish.test.sh refuse-lightweight

### C4: A malformed version tag is refused and nothing is published

Check: bin/release-publish.test.sh refuse-malformed

### C5: A tag off main is refused and nothing is published

Check: bin/release-publish.test.sh refuse-off-main

### C6: The workflow runs bin/test and bin/lint before packaging

Check: grep -q 'bin/lint' .github/workflows/release.yml

### C7: The workflow triggers on tags with contents write only

Check: grep -q 'contents: write' .github/workflows/release.yml

### C8: The workflow uses Go 1.25.11 and no pull request trigger

Check: grep -q "1.25.11" .github/workflows/release.yml

### C9: The whole test script passes

Check: bin/release-publish.test.sh

## Verification

Run every check from the repository root, then `bin/test` and `bin/lint`, and
report each exit code. State plainly that the workflow itself was not run.

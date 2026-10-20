# TF-075: bin/release --publish local fallback

## Goal

Let the owner publish from their machine when the workflow is unavailable,
without creating a duplicate release.

## Dependencies

- TF-073
- TF-074

## Scope

Add `bin/release <version> --tag --publish [--allow-unsigned]`. The owner
decided the local path may push the tag. Because a tag push also triggers the
workflow, the order is:

1. Run the TF-073 gates and tagging, then check `gh release view <version>`.
   If a release already exists, stop with a clear message and push nothing.
2. Push the tag (`git push origin <version>`), only with `--publish`.
3. Check `gh release view` again. If the workflow already created the
   release, upload only missing assets through `bin/release-publish` (which is
   idempotent); otherwise publish through it. Print which path ran.

State the race honestly in `docs/release.md`: the workflow may create the
release between the second check and the publish, so both paths are
idempotent instead of exclusive. Say when to use which: the workflow is the
normal path; the local path is for when Actions is unavailable, and the human
should then not wait for or rerun the workflow.

Tests use a fake `gh` and a local bare repository as `origin`.

## Constraints

Without `--publish` nothing is pushed. Automated workers never run the
publish path against a real remote. The script never passes signing flags.
Bash only; no Go changes.

## Success criteria

### C1: Without --publish nothing is pushed

Check: bin/release.test.sh publish-needs-flag

### C2: An existing release stops the push

Check: bin/release.test.sh publish-existing

### C3: The tag is pushed once and assets published when none exists

Check: bin/release.test.sh publish-new

### C4: The race path uploads only missing assets

Check: bin/release.test.sh publish-race

### C5: The whole test script passes

Check: bin/release.test.sh

## Verification

Run every check from the repository root, then `bin/test` and `bin/lint`, and
report each exit code. Do not push to the real origin.

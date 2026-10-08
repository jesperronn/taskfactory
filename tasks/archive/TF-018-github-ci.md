# TF-018: Run tests and lint in GitHub Actions

## Goal

Make GitHub CI execute the same `bin/test` and `bin/lint` checks required of
local workers.

## Dependencies

TF-001 and TF-017 are archived.

## Scope

Add one GitHub Actions workflow for pull requests and pushes to `main`. Check
out the repository, set up the Go version declared in `go.mod`, set up a
supported Node LTS release, install a pinned project-local Prettier development
dependency from a committed lockfile with `npm ci`, then run `bin/test` and
`bin/lint` as separate steps. Keep test and lint failures visible as failing
jobs. Avoid release, deployment, or publishing steps. Document only the commands
needed to reproduce the CI checks locally.

## Constraints

Use the existing wrappers as the source of truth; do not duplicate test
selection in YAML. Keep Prettier a development-only dependency. Do not use
unpinned `npx` registry resolution during checks. Preserve the Go stdlib-first
runtime policy.

## Success criteria

- The workflow triggers on pull requests and pushes to `main` and runs both
  wrappers.
- `npm ci` installs the exact Prettier version recorded in the lockfile;
  `bin/lint` uses the project-local installation without downloading during the
  lint step.
- A failing `bin/test` or `bin/lint` produces a failed CI job; neither failure
  is masked with `continue-on-error` or `|| true`.
- The local equivalents `npm ci`, `bin/test`, and `bin/lint` pass on the
  candidate branch, and the workflow syntax is validated.
- A GitHub run is inspected after the workflow is pushed or a PR is opened; if
  no remote run is available, report that verification as pending, not passed.

## Verification

Run the local CI commands and inspect their exit codes. Validate workflow YAML
with an available parser. Report the exact workflow path and commit; leave the
remote-run check to TF-020.

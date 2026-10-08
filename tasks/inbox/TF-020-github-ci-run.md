# TF-020: Verify the GitHub CI run

## Goal

Confirm that the test and lint workflow added by TF-018 runs successfully on
GitHub.

## Dependencies

TF-018 must be archived and its workflow must be pushed or included in a PR
before this task can enter ready.

## Scope

Inspect a real GitHub Actions run for the TF-018 workflow on a branch or pull
request containing its implementation. Record the run URL, commit SHA, test-step
conclusion, lint-step conclusion, and any failure logs. If a check fails, create
a focused remediation task rather than weakening checks. Do not alter the
workflow solely to make a failed run green without diagnosing the failure.

## Success criteria

- The inspected run is for the exact TF-018 implementation commit or a
  descendant that contains it.
- Both `bin/test` and `bin/lint` steps completed successfully in that run.
- The task records the run URL, SHA, step conclusions, and the date checked.

## Verification

Use GitHub's run details for the relevant commit and compare its SHA with local
Git history. Report any missing run as pending, not passed.

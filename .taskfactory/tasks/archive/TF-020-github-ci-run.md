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

## Result

Verified by the owner by hand on 2026-10-20 from the GitHub Actions run page;
the commit relationship was checked locally.

- Run: <https://github.com/jesperronn/taskfactory/actions/runs/39383057406>
  (job 118496361197), workflow CI, run number 26, "docs: archive completed
  TF-067", on `main`. It succeeded in 1m 14s, about 13 hours before the check.
- Commit SHA in the run log: `8a879feaa1e48166930cd2ed687cc8f8f41aee0c`, equal
  to the `origin/main` tip at the time of checking.
- The TF-018 commit `e25888891a0559668f2a5c68540ca43716ee4ef0` ("ci: run
  TaskFactory checks on GitHub Actions") is an ancestor of that SHA
  (`git merge-base --is-ancestor` exit 0).
- Step conclusions: Run tests (`bin/test`) succeeded: all shell tests and
  `go test ./...` passed, then whole-tree validation printed "7 task file(s)
  valid" and the summary "all selected checks passed". Build binary
  (`bin/build`) succeeded ("built bin/out/taskfactory (version 8a879fe)").
  Run lint (`bin/lint`) succeeded ("checked=85 failures=0", "7 task file(s)
  valid").
- Toolchain in the run: Go 1.25.11 as set in the workflow; the runner's Git
  defaulted to `master`, which the test fixtures now handle.
- One annotation (a notice) was shown on the run page; its text was not
  recorded.

Success criteria: all three met.

Also seen by the owner on the Actions page, 2026-10-20: CI runs #19 to #26 on
`main` all completed successfully (the latest, #26, is the run above), and the
Dependabot Updates runs for `github_actions` and `go_modules` (#9 to #12, on
2026-10-09 and 2026-10-15) completed successfully. That verifies the
Dependabot configuration from TF-036 on GitHub; there is no `npm` entry, which
matches `.github/dependabot.yml` since Prettier was dropped.

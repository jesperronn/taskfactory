# TF-019: Migrate task contracts to the v1 format

## Goal

Make the repository task tree consistent with the state-specific contract that
TF-003 will validate.

## Dependencies

- TF-006

## Scope

Clarify `docs/task-format-v1.md` for inbox, ready, active, failed, and archived
files. Preserve the protocol's rule that inbox stories may be incomplete; define
the minimum identity and structural checks for them. Require the full executable
contract on promotion to ready. Define how existing archived records with
missing claim data are accepted without inventing history. Migrate current task
files to the appropriate state contract while preserving their intent, IDs,
dependencies, and lifecycle locations. Update TF-003's implementation contract
if clarification changes its required behavior.

## Constraints

Do not implement the validator or move tasks between state directories. Do not
fabricate claims, check results, or completion evidence. Keep the grammar small
and directly parseable by Go's standard library.

## Success criteria

### C1: The five state rules and archive exceptions pass manual review

Check: cat docs/task-format-v1.md docs/protocol-v1.md

### C2: Every task file passes a manual audit against its current state rule

Check: find tasks -type f -exec cat {} +

### C3: The migration preserves every task ID, path, and state

Check: git diff --name-status -M dc5611e HEAD -- tasks

### C4: Repository tests pass

Check: bin/test

### C5: Markdown formatting passes

Check: bin/lint

### C6: The patch has no whitespace errors

Check: git diff --check

## Verification

Manually review the output of the C1-C3 commands against the audit and exact
inventories below; this manual acceptance determines C1-C3. The repository
checks are supplemental: they verify tests, Markdown formatting, and whitespace,
but do not validate the task contract or prove inventory preservation.

State-rule audit at the rebased base `dc5611e38eccad33b18657433d5fb9e29497999d`
(20 task files): all 9 archive files have the required contract headings and
canonical dependency syntax; legacy success-criteria bullets and historical
Outcome sections are covered by the archive exception, and absent Claim blocks
remain un-inferred. Both ready files have all seven executable-contract
headings, criterion/check pairs, and no Claim block. The 9 inbox files pass the
common identity and encoding rules; TF-016 remains an incomplete proposal and is
not judged as an executable contract. Active and failed contain no task files,
so their rules were reviewed in the format and TF-003 contract but have no
current file fixtures.

Exact task-path inventory before this work, at `167af3a` (20 files): archive:
`TF-001-go-cli-foundation.md`, `TF-005-lint-script.md`,
`TF-006-task-file-contract.md`, `TF-007-config-contract.md`,
`TF-014-lint-error-exit.md`, `TF-015-lint-real-prettier-exit.md`,
`TF-017-test-wrapper.md`, `TF-018-github-ci.md`; inbox:
`TF-003-task-validation.md`, `TF-004-init.md`, `TF-008-status.md`,
`TF-009-atomic-claim.md`, `TF-010-worktree-isolation.md`,
`TF-011-worker-evidence.md`, `TF-012-ff-only-integration.md`,
`TF-013-stop-the-line.md`, `TF-016-local-worker-delegation.md`,
`TF-020-github-ci-run.md`; ready: `TF-002-config-loader.md`,
`TF-019-migrate-task-contracts.md`.

Exact task-path inventory at rebased base `dc5611e` and after this migration (20
files): archive: `TF-001-go-cli-foundation.md`, `TF-002-config-loader.md`,
`TF-005-lint-script.md`, `TF-006-task-file-contract.md`,
`TF-007-config-contract.md`, `TF-014-lint-error-exit.md`,
`TF-015-lint-real-prettier-exit.md`, `TF-017-test-wrapper.md`,
`TF-018-github-ci.md`; inbox: `TF-003-task-validation.md`, `TF-008-status.md`,
`TF-009-atomic-claim.md`, `TF-010-worktree-isolation.md`,
`TF-011-worker-evidence.md`, `TF-012-ff-only-integration.md`,
`TF-013-stop-the-line.md`, `TF-016-local-worker-delegation.md`,
`TF-020-github-ci-run.md`; ready: `TF-004-init.md`,
`TF-019-migrate-task-contracts.md`.

The differences between the initial and rebased-base inventories are upstream
lifecycle changes: TF-002 moved from ready to archive in `01a9f46`, and TF-004
moved from inbox to ready in `dc5611e`. Relative to `dc5611e`, this migration
preserves every path and ID. Run `bin/test`, `bin/lint`, and `git diff --check`;
record each exit code.

Acceptance result for this candidate: manual review of C1-C3 passed. The C1
review compared `docs/task-format-v1.md` with `docs/protocol-v1.md`; the C2
review covered all 20 task paths; the C3 inventory comparison found the same 20
paths and unique IDs as `dc5611e`. `bin/test`, `bin/lint`, and
`git diff --check` each exited 0. These results are recorded as the task's
review report, not as worker claim or integration evidence.

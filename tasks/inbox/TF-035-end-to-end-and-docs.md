# TF-035: End-to-end run and user docs

## Goal

Prove the full loop works and document it. What remains is a verbatim re-run
of the quick start, the requeue branch and the CI signal.

## Dependencies

- TF-013
- TF-020
- TF-033

## Scope

Delivered:

- README quick start (install, init, plan, promote, claim, work, verify,
  integrate, check-main, fail): TF-057 (`README.md`, "Quick start").
- One real task end to end through a local worker in a scratch repo, with
  commands, exit codes and the integration commit: TF-057
  (`docs/experiments/e2e-local-worker.md`, integration commit `7f42701`).
- Remediation tasks for the friction found: TF-060 (worker instructions
  fallback) and TF-061 (integrate allows the log directory), both archived.
- Verbatim re-run of the quick start with zero workarounds, a real local
  model and an integration commit in the record:
  `docs/experiments/quickstart-verbatim-rerun-2.md`.
- README fixes from that re-run: TF-064 (stall recipe for the fail branch)
  and TF-065 (worktrees and branches remain after integrate and fail).

Remaining:

- Document the `requeue` branch in the quick start once TF-049 lands. Blocked
  on TF-049 via TF-033.
- TF-020 (CI run on GitHub) as a prerequisite signal; still in inbox.

## Success criteria

- The quick start is followed verbatim in a scratch repo without manual
  fixes. Done: `docs/experiments/quickstart-verbatim-rerun-2.md` (zero
  workarounds, real local model, worker commit and integration in the record).
- Experiment record includes commands, exit codes and the integration commit.
  Done: `docs/experiments/e2e-local-worker.md` (TF-057) and
  `docs/experiments/quickstart-verbatim-rerun-2.md`.
- The quick start documents the `requeue` branch. Open: blocked on TF-049.

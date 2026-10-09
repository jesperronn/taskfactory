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

Remaining:

- Re-run the quick start verbatim in a scratch repo, now that TF-060 and
  TF-061 are delivered, and record it. The TF-057 record states the verbatim
  criterion was not met (manual steps 8 and 13).
- Document the `requeue` branch in the quick start once TF-049 lands. Blocked
  on TF-049 via TF-033.
- TF-020 (CI run on GitHub) as a prerequisite signal; still in inbox.

## Success criteria

- The quick start is followed verbatim in a scratch repo without manual
  fixes. Open: not yet re-run after TF-060 and TF-061.
- Experiment record includes commands, exit codes and the integration commit.
  Done: `docs/experiments/e2e-local-worker.md` (TF-057); the re-run needs its
  own record.

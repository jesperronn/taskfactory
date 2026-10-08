# TF-029: End-to-end run and user docs

## Goal

Prove the full loop works and document it.

## Dependencies

- TF-013
- TF-016
- TF-027

## Scope

Rewrite the README as a quick start (install, init, plan, dispatch worker,
verify, integrate). Run one real task end to end through a local worker in a
scratch repo, record the transcript under docs/experiments, and file remediation
tasks for any friction found. Also covers TF-020 (CI run on GitHub) as a
prerequisite signal.

## Success criteria

- The quick start is followed verbatim in a scratch repo without manual fixes.
- Experiment record includes commands, exit codes and the integration commit.

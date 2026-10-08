# TF-046: Share adapter code and cover the stall path

## Goal

Remove the duplicated launch plumbing in the three local worker adapters and
close the test gaps found when they were verified.

## Dependencies

- TF-024, TF-025 and TF-026 are archived.

## Scope

The packages `internal/adapter/omp`, `pi` and `claude` each define the same
`State`, `Result`, preflight result types, TCP dial helper and run skeleton.
Extract the shared parts into `internal/adapter/common` and keep the harness
specific argv, preflight listing and notes in each package. Add the missing
tests: a stall (timeout) test for the omp and claude packages, matching the Pi
package's `TestRunStallsAtAdapterTimeout`, and a Pi preflight test fed with a
fixture copied from the real `pi --list-models` format (columns provider,
model, context, max-out, thinking, images).

## Constraints

Do not change any adapter argv, flag or note text beyond moving code. The three
named tests of each task must keep passing unchanged. Never start a real model in
tests.

## Notes

Open decisions that affect these adapters, recorded for the owner: the Claude
adapter uses accept-edits permissions while `docs/local-workers-v1.md` says
permissions are set manually, and TF-028 still needs a mapping from a stall to
the evidence outcome enum. Unrefined; promote once those are answered.

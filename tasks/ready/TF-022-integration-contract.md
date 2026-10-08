# TF-022: Define serialized integration evidence and state

## Goal

Give TF-012 a precise, reviewable contract for integrating one verified worker
result into local main.

## Dependencies

- TF-010
- TF-011

## Scope

Specify in `docs/protocol-v1.md` and `docs/technical-spec-v1.md` how an active
task's latest PASS evidence, branch, result commit, and clean worktree qualify
it for integration. Define lock ordering with concurrent claims, local main
movement, rebase and re-verification, direct `git merge --ff-only`, optional
post-merge main checks, and when the task moves to archive. Define how lifecycle
state is committed without staging unrelated tasks or files. Define a minimal
append-only integration attempt record with exact path, keys, types, stages, and
failure semantics, separate from worker evidence. Update TF-012's scope and
success criteria to match.

## Constraints

Do not implement integration or move lifecycle files. Keep the design local to
Git and filesystem operations; no remote fetch, push, database, daemon, or new
dependency. Preserve stop-the-line behavior for a post-merge main failure and
leave persistent recovery to TF-013.

## Success criteria

### C1: Eligibility and exact verified commit rules are unambiguous

Check: cat docs/protocol-v1.md docs/technical-spec-v1.md

### C2: Lock, rebase, ff-only, and archive ordering is explicit

Check: cat docs/protocol-v1.md docs/technical-spec-v1.md

### C3: Integration evidence has an exact schema and examples for pass and failure

Check: cat docs/protocol-v1.md docs/technical-spec-v1.md

### C4: TF-012 has executable acceptance criteria for the contract

Check: cat tasks/inbox/TF-012-ff-only-integration.md

### C5: Repository checks pass

Check: bin/test

## Verification

Manually inspect C1-C4 against the cited files and record any ambiguity. Parse
each JSON example with a real JSON parser and compare keys and types to the
written schema. Run `bin/test`, `bin/lint`, and the task validator; report exact
exits. Automated checks supplement the manual contract review.

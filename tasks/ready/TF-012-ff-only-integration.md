# TF-012: Integrate one verified candidate

## Goal

Serialize candidate integration onto current local main with rebase,
verification, and an ff-only merge.

## Dependencies

- TF-010
- TF-011
- TF-022

## Scope

Implement `taskfactory integrate <ID>` according to the eligibility, locking,
evidence, rebase/recheck, fast-forward, optional main verification, archive, and
failure contract in `docs/protocol-v1.md` and `docs/technical-spec-v1.md`. Use
only local `refs/heads/main`; do not fetch or push. Hold
`.taskfactory/integration.lock` through checks, merge, lifecycle commit,
evidence append, and stop-state handling. Require a latest worker PASS whose
result commit exactly matches clean candidate HEAD. Rebase and rerun all
integration checks if main moves before merge. For a separate lifecycle commit,
stage only the tracked ready deletion plus archive addition when the active
claim is untracked, or the active deletion plus archive addition when active is
tracked. A post-merge main-check failure leaves main advanced and the task
active. When `integration.stop_on_main_failure` is true, create the exact stop
record in the protocol; otherwise retain failure evidence without a stop record.
Reject future integrations while a valid or malformed stop file or its `.tmp`
sibling exists. TF-013 adds `check-main` recovery. Append exact, append-only
integration evidence for every attempt.

## Constraints

Do not contact remotes, run workers, edit candidate feature code, reset main, or
stage unrelated files. Use direct Git subprocesses and the Go standard library.
Preserve existing worker evidence and unrelated task state.

## Success criteria

### C1: Ineligible candidates and unrelated main changes cannot advance main

Check: go test ./internal/integrate

### C2: Concurrent integrations serialize and stale candidates are reverified

Check: go test ./internal/integrate

### C3: Rebase, candidate-check, and ff-only failures retain active state and failure evidence

Check: go test ./internal/integrate

### C4: A passing candidate fast-forwards main and commits only its archive transition

Check: go test ./internal/integrate

### C5: A failed post-merge main check records exact advanced state and a conditional stop

Check: go test ./internal/integrate

### C6: Repository checks pass

Check: bin/test

## Verification

Use temporary local Git repositories with real branches and worktrees. Test
missing or mismatched latest PASS evidence, dirty candidate, absent local main,
unrelated main files, runtime allowlist, and tracked clean config. Inspect
commit ancestry and `git worktree list --porcelain` for passing and stale-main
cases. Force rebase, command, merge, evidence-append, archive-commit, and
post-merge-main failures; compare exact main refs, task paths, stop records, and
append-only evidence bytes. Run `bin/test`, `bin/lint`, `go vet ./...`, and the
task validator; report exact exits and unresolved limits.

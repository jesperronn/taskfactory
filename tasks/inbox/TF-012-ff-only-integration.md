# TF-012: Integrate one verified candidate

## Goal

Serialize candidate integration onto current main with rebase, verification, and
ff-only merge.

## Dependencies

TF-010, TF-011, and TF-022 must be archived before promotion to ready.

## Scope

Implement `taskfactory integrate <ID>` according to the exact eligibility,
locking, evidence, rebase/recheck, fast-forward, optional main verification,
archive, and failure contract in `docs/protocol-v1.md` and
`docs/technical-spec-v1.md`. Use only local `refs/heads/main`; do not fetch or
push. Hold `.taskfactory/integration.lock` through checks, merge, lifecycle
commit, evidence append, and stop-state handling. Require a latest worker PASS
whose result commit exactly matches clean candidate HEAD. Rebase and rerun all
integration checks if main moves before merge. Keep task lifecycle commits
separate from the feature fast-forward and stage only this task's paths. A
post-merge main check failure leaves main advanced and the task active, then
uses TF-013's persistent stop state when `integration.stop_on_main_failure` is
true; when false, retain the evidence without creating a stop record. Never
claim rollback. Append exact, append-only integration evidence for successful
and failed attempts.

## Success criteria

- `integrate` rejects a missing/mismatched latest worker PASS, a dirty
  candidate, unrelated main-worktree changes, absent local main, or a persistent
  TF-013 stop record without advancing main. Check: go test ./internal/integrate
  ./internal/claim
- Two simultaneous integrations serialize on the integration lock. A main ref
  change before merge causes rebase and a fresh full integration-check sequence;
  only the exact checked commit is fast-forwarded. Check: go test
  ./internal/integrate
- Rebase, integration-check, and ff-only failures leave the task active and main
  unchanged, with one schema-valid failure record per attempt. Check: go test
  ./internal/integrate
- Passing integration and configured main checks produce a fast-forward, then a
  separately committed archive transition with only this task's paths staged and
  one schema-valid PASS record. Check: go test ./internal/integrate
- With `integration.stop_on_main_failure = true`, a post-merge main-check
  failure leaves main at the merged commit, task active, and TF-013 stop state
  persistent; a failed evidence append never rewrites older records. Check: go
  test ./internal/integrate
- `go test ./...` and `go vet ./...` pass. Check: go test ./... && go vet ./...

## Verification

Use temporary local Git repositories for success, stale-main, and failure cases;
inspect commit ancestry, task state, and evidence.

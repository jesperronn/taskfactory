# TF-013: Stop integration when main is broken

## Goal

Prevent ordinary integration while main fails its required checks and retain
repair evidence.

## Dependencies

TF-012 must be archived before promotion to ready.

## Scope

Extend TF-012's `.taskfactory/integration-stop.json` record and reject behavior
without changing its path or key schema. When main verification fails and
`integration.stop_on_main_failure` is true, record the exact main commit,
command, exit code, output, and start error. Ordinary `integrate` remains
rejected while the stop file exists; isolated workers may continue. If an
interrupted atomic write left only `.taskfactory/integration-stop.json.tmp`,
validate it with the same schema and treat a valid record as stopped; if it is
malformed, fail closed and require manual inspection. Add
`taskfactory check-main` to rerun configured main verification on the current
main commit. On failure, atomically write or update the canonical record with
the current commit and failed command result, retaining the stop. Clear the
canonical record and any valid leftover temp only after all checks pass; do not
clear it merely because main moved. Keep repair-task prioritization with the
orchestrator.

## Success criteria

- When `integration.stop_on_main_failure` is true, failed main checks stop
  subsequent ordinary integrations without moving their task files.
- `check-main` reads `.taskfactory/integration-stop.json` using the exact schema
  in `docs/protocol-v1.md`; it updates the existing stop record on a failed
  recheck and removes it only after current-main verification passes. If only
  the `.tmp` sibling exists, it treats a valid temp record as stopped and clears
  it only after a passing recheck; a malformed temp remains for manual
  inspection.
- A failed recheck retains the stop; a passing `taskfactory check-main` clears
  it and permits subsequent ordinary integration.
- Post-merge failure never archives its task as successful.
- `go test ./...` and `go vet ./...` pass.

## Verification

Exercise red-main, post-merge failure, rejected integration, and successful
recovery in temporary Git repositories.

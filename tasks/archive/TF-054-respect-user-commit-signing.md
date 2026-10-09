# TF-054: Respect the user's commit signing configuration

## Goal

TaskFactory commands that create commits run plain `git commit` and respect
the user's own Git configuration. The owner handles commit signing and keeps
control of it, so TaskFactory must not override signing with
`--no-gpg-sign`, `-c commit.gpgsign=false`, or any similar flag.

## Dependencies

- TF-047

## Scope

Remove the signing override from the non-test call site that creates commits:

- `internal/promote/promote.go` `commitPaths`: drop `--no-gpg-sign` from the
  `git commit --quiet --only -m ... --` call.

Other TaskFactory commit call sites were checked and already run plain
`git commit` with no signing override: `internal/integrate/integrate.go`
(archive commit, `commit -m`). The claim, verify, and cmd packages create no
commits in production code.

Test fixtures in every package that creates a temporary repository and commits
in it are updated to set `commit.gpgsign` to `false` in that repository's own
configuration, so tests stay hermetic:

- `internal/promote/promote_test.go` (already sets it; the `commit` helper's
  `--no-gpg-sign` flag is removed)
- `internal/integrate/integrate_test.go`, `internal/integrate/checkmain_test.go`
- `internal/verify/verify_test.go`
- `internal/claim/operation_test.go`
- `cmd/taskfactory/main_test.go`, `cmd/taskfactory/promote_test.go`

Documentation and help text that describe the commit behavior are updated:

- `cmd/taskfactory/main.go` promote help text
- `README.md`
- `docs/worker-instructions.md`
- `skills/taskfactory/SKILL.md`

## Constraints

- Production Go code must not contain `no-gpg-sign`, `commit.gpgsign`, or any
  `-c` override that changes signing. Tests may set repository configuration.
- Keep behavior otherwise identical: the same staging, the same `--only`
  commit scope, the same commit message, and the same rollback on failure.
- Test fixtures stay hermetic: they set `commit.gpgsign` to `false` in their
  own temporary repository configuration and do not depend on the developer's
  global Git configuration.
- Docs state that TaskFactory commits follow the user's Git signing
  configuration, and that automated workers run in an environment where
  signing is configured to work or is explicitly disabled by the environment
  owner, not by TaskFactory.

## Success criteria

### C1: Promotion honors the user's signing configuration

Check: go test ./internal/promote -run Signing -v

### C2: No production Go source overrides commit signing

Check: go test ./internal/promote -run TestSigningOverrideAbsent -v

### C3: The whole Go module still passes

Check: go test ./...

## Verification

Run each check from the repository root and report its exit code and the
relevant output. Also run `bin/test`, `bin/lint`, `go vet ./...`,
`gofmt -l .`, and `go run ./cmd/taskfactory validate tasks`, and report each
exit code.

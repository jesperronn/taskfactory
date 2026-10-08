# TF-002: Load and validate project configuration

## Goal

Implement the v1 Go configuration loader using the approved TOML contract.

## Dependencies

TF-001 and TF-007 must be archived before promotion to ready.

## Scope

Add a focused `internal/config` package with a
`Load(projectRoot string) (Config, error)` entry point for loading
`.taskfactory/config.toml` from a project root. Implement the exact keys,
defaults, rejection rules, and parser choice in `docs/config-v1.md`. Add
table-driven tests for a valid full config, defaults, unknown keys, bad types,
unsupported protocol version, and worker limits 0 and 5. Do not implement CLI
`init`, task parsing, or Git operations. Missing config is an error rather than
an implicit default project.

## Success criteria

- The loader returns explicit errors naming the offending key or missing file.
- The documented valid config loads with maximum parallel workers 4 and ff-only
  integration.
- Invalid examples from `docs/config-v1.md` are rejected; errors identify the
  key and source file.
- `go test ./...` and `go vet ./...` pass.

## Verification

Run focused loader tests, `go test ./...`, and `go vet ./...`. Report dependency
changes and the exact TOML parser used.

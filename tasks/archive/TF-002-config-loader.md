# TF-002: Load and validate project configuration

## Goal

Implement the v1 Go configuration loader using the approved TOML contract.

## Dependencies

- TF-001
- TF-007

## Scope

Add a focused `internal/config` package with a
`Load(projectRoot string) (Config, error)` entry point for loading
`.taskfactory/config.toml` from a project root. Implement the exact keys,
defaults, rejection rules, and parser choice in `docs/config-v1.md`. Add
table-driven tests for a valid full config, defaults, unknown keys, bad types,
unsupported protocol version, and worker limits 0 and 5. Do not implement CLI
`init`, task parsing, or Git operations.

## Constraints

A missing config is an error rather than an implicit default project.

## Success criteria

### C1: Loader errors identify invalid or missing configuration

Check: go test ./...

### C2: The valid config loads with the documented defaults and policy

Check: go test ./...

### C3: Invalid examples are rejected with key and source file identified

Check: go test ./...

### C4: Go tests and vet pass

Check: go test ./... && go vet ./...

## Verification

Run focused loader tests, `go test ./...`, and `go vet ./...`. Report dependency
changes and the exact TOML parser used.

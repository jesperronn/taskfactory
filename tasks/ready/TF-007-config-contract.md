# TF-007: Define the v1 TOML configuration contract

## Goal

Make project configuration sufficiently precise for a small Go loader and
deterministic `init` output.

## Dependencies

None.

## Scope

Create `docs/config-v1.md`. Specify the exact v1 TOML keys and types for
protocol version, worker limit, worktree policy/location,
worker/integration/optional main verification commands, and integration policy.
Define defaults, invalid values, unknown-key behavior, and config lookup
relative to a project root. Provide one complete valid example and at least
three invalid examples with expected errors. Choose whether v1 uses one mature
TOML module or a bounded subset parser; record the maintenance rationale. Do not
add Go code, `go.mod`, or an actual project config.

## Constraints

Follow `docs/technical-spec-v1.md`: Go, standard library first, near-zero
dependencies, maximum four workers by default, worktrees, and ff-only
integration. Treat configuration as project policy, never task state.

## Success criteria

- Every key in the technical spec's example has a defined type and validation
  rule.
- Defaults and unsupported protocol versions are explicit.
- The worker limit cannot exceed four in v1; invalid limits have a defined
  error.
- The document defines how worker, integration, and optional main verification
  commands are represented and executed, including whether a shell is involved.
  It specifies the per-task worktree path used by claim.
- A future `init` implementation can emit the example without further format
  decisions.

## Verification

Compare the document against the configuration example in
`docs/technical-spec-v1.md` and run `git diff --check` on this task's changes.
Record any spec inconsistency discovered rather than changing unrelated files.

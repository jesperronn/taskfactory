# TaskFactory technical specification v1

## Purpose and requirement criteria

Build a small, reusable `taskfactory` CLI for the [protocol](protocol-v1.md), usable in arbitrary Git projects and independent of an AI provider. The implementation criteria are:

- Small, simple code with few files, descriptive names, limited indirection, explicit errors, and no hidden state.
- Easy long-term maintenance and upgrades to newer Go/Git versions; avoid undocumented Git behavior, deprecated APIs, and unnecessary platform assumptions.
- Strong testability with deterministic filesystem tests and real temporary Git repositories where Git behavior matters.
- Standard library first and zero or near-zero third-party runtime dependencies. Each dependency needs a concrete maintenance justification.
- Straightforward eventual packaging as a standalone executable, with no required runtime on the target machine. Public distribution infrastructure is deferred.
- Concise, actionable CLI output and meaningful nonzero exit codes on failure.

## Implementation shape

Use Go. Keep a conventional, small layout: `cmd/taskfactory/main.go` and only the `internal/` packages that prove useful. Prefer direct filesystem operations, `os/exec` calls to Git, `flag`, and `testing` over frameworks. Do not add a daemon, API, database, plugin system, event bus, RPC, or logging framework. Avoid goroutines unless bounded concurrency needs them.

Declare the supported Go version in `go.mod` when implementation begins. Target macOS and Linux initially; do not needlessly preclude Windows. Invoke Git directly rather than depending on OS shell syntax for Git operations. The CLI must work outside its own source checkout.

## Configuration and task files

An initialized project has `.taskfactory/config.toml` and `tasks/{inbox,ready,active,failed,archive}`. TOML holds project policy, not transient task state. Keep its surface narrow: protocol version, maximum parallel workers (default 4), worktree location/behavior, verification commands, and integration policy. Representative policy:

```toml
protocol_version = 1

[workers]
max_parallel = 4

[git]
use_worktrees = true
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = ["go test ./..."]
integration = ["go test ./...", "go vet ./..."]
```

Go has no standard-library TOML parser. Prefer one mature, narrowly scoped TOML dependency over maintaining a general parser if parsing cannot remain simple. Decide this before implementing config loading. Repository-defined verification commands are trusted; execute them transparently, preserve exit status, and capture enough output for evidence. Task contracts are human-readable; choose and document a reliably validated on-disk task format before implementing `validate` or `claim`.

## CLI and milestones

Use the standard `flag` package initially. Support global `--help` and `--version`. The first milestone should grow in small, independently tested slices toward `init`, `status`, `validate`, and `claim`, with configuration loading, task validation, atomic claiming, and worktree creation. Later slices add worker verification and serialized `integrate`. Do not build release infrastructure yet.

Implement the protocol's invariants: one owner per claim, resolved dependencies, max four workers by default, separate worktrees, worker verification/repair evidence, serialized integration, rebasing against current main, `git merge --ff-only`, and stop-the-line when main is broken. Never archive solely on worker success.

## Tests and acceptance

Use Go `testing`, table-driven cases where helpful, `t.TempDir()`, and temporary local Git repositories for claim/worktree/rebase/integration behavior. Avoid excessive mocking. Test invalid transitions and failures as well as success. Before a feature is considered complete, run its relevant tests and `go test ./...`; run `go vet ./...` for integration-level changes. This bootstrap contains specifications and task contracts only, so it does not create `go.mod` or pretend those commands pass yet.

The CLI should report protocol violations with the task ID and next useful fact, for example an unarchived dependency. Default output is concise and predictable. JSON output is optional when orchestration shows a concrete need.

## Explicit non-goals

No web UI, agent-provider integration, model-selection engine, cloud service, Kubernetes integration, telemetry platform, automatic dependency installation, or public release mechanism in the initial implementation. The separate `taskfactory-examples` repository can later host very small end-to-end examples.

# TaskFactory technical specification v1

## Purpose and requirement criteria

Build a small, reusable `taskfactory` CLI for the [protocol](protocol-v1.md),
usable in arbitrary Git projects and independent of an AI provider. The
implementation criteria are:

- Small, simple code with few files, descriptive names, limited indirection,
  explicit errors, and no hidden state.
- Easy long-term maintenance and upgrades to newer Go/Git versions; avoid
  undocumented Git behavior, deprecated APIs, and unnecessary platform
  assumptions.
- Strong testability with deterministic filesystem tests and real temporary Git
  repositories where Git behavior matters.
- Standard library first and zero or near-zero third-party runtime dependencies.
  Each dependency needs a concrete maintenance justification.
- Straightforward eventual packaging as a standalone executable, with no
  required runtime on the target machine. Public distribution infrastructure is
  deferred.
- Concise, actionable CLI output and meaningful nonzero exit codes on failure.

## Implementation shape

Use Go. Keep a conventional, small layout: `cmd/taskfactory/main.go` and only
the `internal/` packages that prove useful. Prefer direct filesystem operations,
`os/exec` calls to Git, `flag`, and `testing` over frameworks. Do not add a
daemon, API, database, plugin system, event bus, RPC, or logging framework.
Avoid goroutines unless bounded concurrency needs them.

Declare the supported Go version in `go.mod` when implementation begins. Target
macOS and Linux initially; do not needlessly preclude Windows. Invoke Git
directly rather than depending on OS shell syntax for Git operations. The CLI
must work outside its own source checkout.

## Configuration and task files

An initialized project has `.taskfactory/config.toml` and
`tasks/{inbox,ready,active,failed,archive}`. TOML holds project policy, not
transient task state. Keep its surface narrow: protocol version, maximum
parallel workers (default 4), worktree location/behavior, verification commands,
and integration policy. Representative policy:

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

Go has no standard-library TOML parser. Prefer one mature, narrowly scoped TOML
dependency over maintaining a general parser if parsing cannot remain simple.
Decide this before implementing config loading. Repository-defined verification
commands are trusted; execute them transparently, preserve exit status, and
capture enough output for evidence. Task contracts are human-readable; choose
and document a reliably validated on-disk task format before implementing
`validate` or `claim`.

## CLI and milestones

Use the standard `flag` package initially. Support global `--help` and
`--version`. The first milestone should grow in small, independently tested
slices toward `init`, `status`, `validate`, and `claim`, with configuration
loading, task validation, atomic claiming, and worktree creation. Later slices
add worker verification and serialized `integrate`. Do not build release
infrastructure yet.

`taskfactory validate` with no arguments checks the inbox and ready task files.
Paths given as arguments are task files or folders under `tasks/`;
`taskfactory validate tasks` checks the whole tree. Tree-wide ID and dependency
checks always run, but only diagnostics for the selected files are reported.

Implement the protocol's invariants: one owner per claim, resolved dependencies,
max four workers by default, separate worktrees, worker verification/repair
evidence, serialized integration, rebasing against current main,
`git merge --ff-only`, and stop-the-line when main is broken. Never archive
solely on worker success.

### Integration contract

`integrate <ID>` operates only on local `refs/heads/main`; it does not fetch or
push. Require local main to exist and be checked out in the repository worktree.
The main worktree may contain ready-to-active claim transitions (each ready task
file moved to active with its Claim block), TaskFactory's exact runtime paths
from `protocol-v1.md`, and registered worker worktrees under the configured
root. Reject unrelated tracked or untracked changes, and never stage concurrent
claim transitions. Inventory runtime directories even when Git ignore rules hide
them, so ignored user files are not mistaken for TaskFactory state. The task
worktree itself must be clean. `.taskfactory/config.toml` is allowed only when
tracked in HEAD and clean in the index and worktree; reject a missing,
untracked, staged, or unstaged config. Recheck it immediately before merge and
before lifecycle staging. Never stage or commit it. The claimed branch must be
checked out in its registered worktree, and the complete, schema-valid,
consecutively numbered worker evidence file must end with a PASS for the claimed
task and branch. Its `base_commit` must match Claim metadata, its
`result_commit` must equal candidate HEAD, and that commit must descend from the
recorded base. A later FAILED or BLOCKED worker record invalidates an earlier
PASS; malformed or incomplete evidence is not skipped.

Serialize integrations with `.taskfactory/integration.lock`. Claims serialize
only with `.taskfactory/claim.lock`; neither operation acquires both locks, so
there is no nested lock order. A claim may read main before an integration
advances it; the candidate is rebased and verified against current main when
integrated. Under the integration lock, reject a valid or malformed
`.taskfactory/integration-stop.json` fail-closed, rebase onto main, run
`verification.integration` in order at the candidate root, reread main
immediately before the merge, and repeat rebase plus all integration checks if
main moved. Fast-forward the checked-out local main with
`git merge --ff-only <verified-commit>`. Never create a merge commit or reset
main to undo a post-merge failure. The lock coordinates TaskFactory processes;
external Git processes do not honor it.

Run optional `verification.main` at the repository root after the fast-forward.
A failure records the advanced main commit and leaves the task active. When
`integration.stop_on_main_failure` is true, TF-012 atomically writes the exact
stop record defined in `protocol-v1.md`; later integrations reject while it
exists. TF-013 adds `check-main` and clears that record only after all
configured checks pass on current main. When the setting is false, retain
failure evidence without creating a stop record. After all checks pass, move
only this task from active to archive and commit the lifecycle transition
separately. Claims are uncommitted ready-to-active working-tree transitions: if
active is untracked, stage only the tracked ready deletion and archive addition;
if active is tracked, stage only its active deletion and archive addition. Scope
Git staging to those exact paths. If the config changed after main advanced,
restore the task to active and record an archive failure. Never stage unrelated
task or user files. Append a PASS integration record after the archive commit;
if that commit fails, append an archive failure and leave main advanced. If the
final evidence append fails, preserve the already committed archive and report
the missing evidence; do not rewrite existing JSONL.

Append one integration-attempt object per attempt to
`.taskfactory/integration-evidence/<ID>.jsonl`, independently from worker
evidence. Each object has exactly the schema and examples in
[`protocol-v1.md`](protocol-v1.md): task and attempt identity, timestamp,
outcome/stage, branch, worker/base/main/verified commit IDs, failed command and
exit status, captured output/error, and note. Earlier lines are immutable.
Failures before merge leave main and task lifecycle state unchanged; failed
post-merge main checks are the explicit exception and must never be described as
rolled back.

## Tests and acceptance

Use Go `testing`, table-driven cases where helpful, `t.TempDir()`, and temporary
local Git repositories for claim/worktree/rebase/integration behavior. Avoid
excessive mocking. Test invalid transitions and failures as well as success.
Before a feature is considered complete, run its relevant tests and
`go test ./...`; run `go vet ./...` for integration-level changes. This
bootstrap contains specifications and task contracts only, so it does not create
`go.mod` or pretend those commands pass yet.

The CLI should report protocol violations with the task ID and next useful fact,
for example an unarchived dependency. Default output is concise and predictable.
JSON output is optional when orchestration shows a concrete need.

## Explicit non-goals

No web UI, agent-provider integration, model-selection engine, cloud service,
Kubernetes integration, telemetry platform, automatic dependency installation,
or public release mechanism in the initial implementation. The separate
`taskfactory-examples` repository can later host very small end-to-end examples.

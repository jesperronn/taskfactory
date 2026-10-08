# Local workers v1

The minimal launch and result interface for a local worker adapter. It turns a
single ready task into one bounded adapter run: TaskFactory passes the task and
its claimed checkout path to a harness, observes progress, retains the result,
and independently verifies the worker's claim.

This document describes **observed local CLI behavior** for OMP v18.8.5, Pi, and
Claude Code routed through oMLX, as recorded in
`docs/experiments/TF-015-worker-comparison.md`. Where a harness does not expose
a flag locally, that gap is marked _planned_ and kept out of the launch example.
The three harnesses do **not** expose identical flags; each adapter maps the
contract to its own flags. Worker selection is manual in v1.

## Launch contract

A launch request carries exactly these fields. Every field is explicit; there is
no auto-detection of the adapter and no default model.

- `task_id`: The task ID, e.g. `TF-024`.
- `worktree`: Canonical absolute path of the claimed worktree.
- `adapter`: One of `omp`, `pi`, `claude`. Selected by the operator, not
  inferred.
- `model`: Explicit model id, e.g. `ornith1.5-35B`. No fallback.
- `endpoint`: Project-local endpoint: host, port, and how the api-key is
  sourced. Empty for hosted providers; required for an oMLX-backed run.
- `permissions`: Permission mode (see Permissions).
- `timeout`: Duration after which a run is considered stalled.
- `progress`: Observable progress channel (see Observable progress).
- `prompt`: The task body and any files to include.

## Result contract

A run returns exactly one result.

- `state`: `exit`, `blocked`, or `stalled`.
- `exit_code`: Shell exit code (0 = success, non-zero = failure), or null if the
  shell could not start.
- `commit`: Result commit SHA in the worktree, or the empty string if none was
  made.
- `changed_files`: Repository-relative paths changed from the base commit,
  including untracked.
- `evidence`: Path to the appended evidence record
  (`.taskfactory/evidence/<task_id>.jsonl`).
- `progress`: Observed progress/observations from the channel.
- `note`: Optional human context.

## Preflight: refuse an unavailable model or endpoint before launch

Before launching, the adapter runs a preflight and **refuses to start** if any
check fails. A refused run records `state: blocked` and no worktree mutation
beyond a BLOCKED evidence record; it never silently completes or picks another
model.

1. The adapter is installed (`omp`, `pi`, or `claude` resolves on PATH).
2. The model is selectable and present in the local catalog. Observe the
   harness's model list (`omp --models`, `pi --list-models`, `claude` catalog)
   against the requested model. Refuse if it is absent. This is exactly the
   TF-015 outcome where Claude Code "warned that the Ornith model was not in its
   local model catalog" and made no changes.
3. The project-local endpoint is reachable: an oMLX server is running and bound
   to the configured host/port (see Project-local endpoint). Refuse if it is
   not.

A preflight failure records a BLOCKED attempt and returns before launch.

## Explicit model, no fallback

The operator names the model. The adapter must not fall back to Gemma, a
default, or a cheap role model when the requested model is unavailable.

- **OMP** initial example (observed flags):
  `omp --model=ornith1.5-35B --cwd <worktree> -p "..."`. The example explicitly
  selects `ornith1.5-35B`. It does **not** use `--smol`, `--plan`, or `--slow`
  role models, and it does not fall back to Gemma or any default model.
- **Pi** (observed flags): `pi --model ornith1.5-35B --cwd <worktree> -p "..."`.
  Pi matches a `provider/id` pattern; the operator pins the id.
- **Claude Code** (observed flags):
  `claude --model ornith1.5-35B --add-dir <worktree> -p "..."`. The adapter must
  **not** pass `--fallback-model`; doing so reintroduces the fallback this
  contract forbids.

## Project-local endpoint

Local runs point at an oMLX server on loopback rather than a remote provider.

- Start the server: `omlx serve --host 127.0.0.1 --port 8000` (the defaults).
  Models are discovered from subdirectories of `--model-dir` (default
  `~/.omlx/models`).
- Point a coding tool at it: `omlx launch <tool> --host 127.0.0.1 --port 8000`.
  oMLX configures the selected tool (Claude Code, Pi, ...) to use the running
  server. Observed: Claude Code "Connected to `127.0.0.1:8000`".

The endpoint is project-local and loopback. The api-key, if required by the
server, is sourced from the environment or a local config file; it is **not**
stored in a task or evidence record.

## Permissions

Manual permission handling for v1; the adapter does not auto-approve.

- **Claude Code**: `--permission-mode manual` (also
  `--permission-prompts host`).
- **OMP**: omit `--auto-approve`; manual is the default. `--approval-mode`
  exists but is not used to bypass it.
- **Pi**: manual means do not pass `-a`/`--approve`.

## Timeout

A run that makes no progress within `timeout` is recorded as `stalled`, not
archived or completed.

- **OMP**: `--max-time=10m` (observed; accepts `600`, `10m`, `1h`).
- **Claude Code**: observed flag is `--max-budget-usd` (a cost cap, not a time
  bound). A wall-clock timeout here is _planned_; document it as such.
- **Pi**: no timeout flag was observed in help; treat as _planned_.

## Observable progress

The adapter exposes a live progress channel so TaskFactory can observe a run
without reading the model's internal state.

- **Claude Code**: `--print --output-format stream-json` (realtime streaming).
- **OMP**: `--mode rpc` or `--mode rpc-ui`.
- **Pi**: `--mode rpc`.

All three support a non-interactive `--print`/`-p` that processes the prompt and
exits with a shell exit code, which is how `state: exit` is produced.

## exit / blocked / stalled states

- **exit** — the harness's `--print` exits with a shell code: `0` is success,
  any non-zero code is a failure.
- **blocked** — the preflight refused (unavailable model or endpoint). Recorded
  as a BLOCKED attempt before launch; no worktree mutation beyond the evidence.
- **stalled** — no progress within `timeout` (e.g. OMP `--max-time` expiry, or
  an oMLX server that stops responding). Recorded and left in place; it is never
  silently completed or archived.

## Retained commit and evidence paths

- **Commit**: the worker commits its work in the claimed worktree; TaskFactory
  records the result commit SHA. The claimed branch and worktree are preserved
  across completion, failure, stall, stop, and resume.
- **Evidence**: each attempt appends one JSON record to
  `.taskfactory/evidence/<task_id>.jsonl` (schema in `docs/task-format-v1.md`).
  The file is append-only; prior bytes are never changed. A run with no result
  commit records the empty string for `commit`.

Completion, failure, stall, stop, and resume all keep the task's Git state
(branch + worktree) and its evidence intact. A resume reuses the same branch,
worktree, base commit, and appends a new attempt number; a stop records the
point reached without archiving; a stall or failure records `stalled`/`FAILED`
and leaves the worktree for repair or hand-off.

## How the worker runs `bin/test` and `bin/lint`

The worker runs the project's own wrappers inside its claimed worktree as part
of its verification: `bin/test` then `bin/lint`. It records the exact command
and exit status of each into its evidence attempt. A worker pass does not
complete the task; it only supplies evidence for TaskFactory's independent
check.

## How TaskFactory independently calls `verify`

TaskFactory runs `taskfactory verify <ID>` against the claimed worktree. It runs
the task's criterion checks in order, then the configured `verification.worker`
commands, and appends one evidence record per attempt — independently of the
worker's own claims. A task is not marked complete on a failed or blocked check;
retries stay under orchestrator control. This is the independent verification
that a worker's self-reported pass cannot substitute for.

## Split of TF-016

TF-016 is split into small implementation tasks that each implement this
contract for one concern. They stay in `tasks/inbox/` until a harness can launch
a model and verify them; none is promoted to `ready` yet. Each depends on the
archived TF-010 (worktree isolation) and TF-011 (worker evidence); TF-026 also
depends on the TF-015 experiment.

- **TF-024** (inbox): OMP adapter launch with explicit `ornith1.5-35B`, no
  fallback, local endpoint, permissions, timeout, and running
  `bin/test`/`bin/lint`.
- **TF-025** (inbox): Pi adapter launch under the same contract via Pi flags.
- **TF-026** (inbox): Claude Code adapter through the oMLX local endpoint,
  resolving the TF-015 "model not in catalog" warning.
- **TF-027** (inbox): TaskFactory result/evidence recorder and independent
  `verify` call.
- **TF-028** (inbox): Worker state machine for completion, failure, stall, stop,
  and resume preserving Git state and evidence.

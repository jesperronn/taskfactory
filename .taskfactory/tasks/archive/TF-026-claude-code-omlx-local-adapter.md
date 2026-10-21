# TF-026: Claude Code adapter through the oMLX local endpoint

## Goal

Run one claimed task through Claude Code routed to a project-local oMLX server,
with an explicit model and no fallback, and report the run as `exit`,
`blocked`, or `stalled` in the result contract of `docs/local-workers-v1.md`.

## Dependencies

- TF-010
- TF-011
- TF-015

## Scope

Add a Claude Code adapter package at `internal/adapter/claude/` that builds the
launch argv, runs a preflight, and maps the process result to the v1 result
contract. The observed working invocation, from
`docs/experiments/local-harness-trials.md`, passes the prompt on stdin:

`claude -p --disallowedTools LSP --permission-mode acceptEdits --allowedTools <list>`

The trials record that `--allowedTools` is variadic and swallows a positional
prompt, so the prompt always goes on stdin. The `<list>` value is not recorded
in the trials. Choose the smallest tool list that lets a task run `bin/test`
and `bin/lint`, and record it in the adapter's tests.

The endpoint is `omlx serve --host 127.0.0.1 --port 8000`, with Claude Code
pointed at it by `omlx launch claude --host 127.0.0.1 --port 8000`, per
`docs/local-workers-v1.md`. The trials show a "model not in catalog" warning
for Ornith names alongside a run that completed. The warning alone must not
cause a refusal. The preflight refuses when `omlx launch --help` or
`claude --help` shows that the requested model is not registered for the local
endpoint. The worker records which help output decided the outcome.

## Constraints

- Select the Ornith model explicitly and never pass `--fallback-model`. The
  trials show the `haiku` alias selects the 9B model for side calls. The result
  note must say so, and a run must not be described as 35B-only.
- The model selection flag and the `ANTHROPIC_*` environment variables are not
  recorded in `docs/local-workers-v1.md` or the trials. Take them from
  `claude --help` and the operator's environment. Never invent their values and
  never write them into task or evidence files.
- Permissions: the trials used `--permission-mode acceptEdits`, which is the
  observed working value. `docs/local-workers-v1.md` says `manual`. The adapter
  uses `acceptEdits` and the result note records the conflict for the owner to
  reconcile.
- A wall-clock timeout is planned, not implemented. `--max-budget-usd` is a
  cost cap, not a time bound. The result note must say so.
- Run one local harness at a time.
- Verification must not launch a model. Tests use a stub `claude` executable
  placed first on PATH.

## Success criteria

### C1: The Claude argv routes to the local endpoint with no fallback

Check: go test ./internal/adapter/claude/... -run TestArgvUsesStdinAndNoFallback

### C2: Preflight refuses an unregistered model or unavailable endpoint

Check: go test ./internal/adapter/claude/... -run TestPreflightRefusesBeforeLaunch

### C3: A stub run creates a file in a temp worktree and reports exit 0

Check: go test ./internal/adapter/claude/... -run TestStubRunCreatesFileInTempWorktree

### C4: The repository checks pass

Check: bin/test

## Verification

Run the three `go test` checks above and `bin/test` from the repository root,
then `bin/lint` and `go run ./cmd/taskfactory validate tasks`. Report each exit
code. Run `claude --help`, `omlx serve --help`, and `omlx launch --help` without
a prompt. Report whether `--permission-mode`, `--allowedTools`,
`--disallowedTools`, and the model-selection flag appear, and quote the help
line that decided the catalog outcome. Report any assumption help did not
confirm.

Run the three named tests with `-v` and confirm each is listed as PASS; a run
that reports no tests to run does not count as a pass.

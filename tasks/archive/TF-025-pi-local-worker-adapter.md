# TF-025: Pi local worker adapter

## Goal

Run one claimed task through Pi against the project-local oMLX endpoint with an
explicit model and no fallback, and report the run as `exit`, `blocked`, or
`stalled` in the result contract of `docs/local-workers-v1.md`.

## Dependencies

- TF-010
- TF-011

## Scope

Add a Pi adapter package at `internal/adapter/pi/` that builds the launch argv,
runs a preflight, and maps the process result to the v1 result contract. The
observed working invocation, from `docs/experiments/local-harness-trials.md`,
is:

`pi --provider omlx --model Ornith-1.5-35B-A3B-MLX-4bit`
`--thinking low --no-session -p <prompt>`

The adapter runs this with the claimed worktree as the working directory and
passes the task body as the prompt. A normal process exit becomes `exit` with
the shell exit code. The preflight returns `blocked` before launch when `pi` is
not on PATH, when nothing listens on 127.0.0.1:8000, or when the model is not
listed by Pi. Run `pi --help` to confirm the model listing command before
writing the preflight.

Permissions: manual means the adapter never passes `-a` or `--approve`.

Progress: `--mode rpc` is listed in `docs/local-workers-v1.md` but was not
observed in a trial. Confirm it with `pi --help`; if absent, progress is the
captured `-p` output and the result note says so.

## Constraints

- Pin the model explicitly. Pi must not pick a model on its own. Omitting
  `--model` is refused.
- Pi works against the `omlx` provider already present in its models file. No
  proxy is needed.
- No timeout flag was observed for Pi. A wall-clock bound is planned, not
  implemented; the result note must say so and the adapter must not claim a
  time bound.
- Run one local harness at a time.
- Verification must not launch a model. Tests use a stub `pi` executable placed
  first on PATH.
- Store no endpoint keys or other secrets in task or evidence files.

## Success criteria

### C1: The Pi argv selects the explicit model with no fallback

Check: go test ./internal/adapter/pi/... -run TestArgvSelectsExplicitModel

### C2: Preflight refuses an unavailable adapter, model, or endpoint

Check: go test ./internal/adapter/pi/... -run TestPreflightRefusesBeforeLaunch

### C3: A stub run creates a file in a temp worktree and reports exit 0

Check: go test ./internal/adapter/pi/... -run TestStubRunCreatesFileInTempWorktree

### C4: The repository checks pass

Check: bin/test

## Verification

Run the three `go test` checks above and `bin/test` from the repository root,
then `bin/lint` and `go run ./cmd/taskfactory validate tasks`. Report each exit
code. Run `pi --help` without a prompt and report whether `--model`,
`--provider`, `--thinking`, `--no-session`, and `-p` appear, and whether a
progress flag and a timeout flag were found. Report any assumption that help
did not confirm.

Run the three named tests with `-v` and confirm each is listed as PASS; a run
that reports no tests to run does not count as a pass.

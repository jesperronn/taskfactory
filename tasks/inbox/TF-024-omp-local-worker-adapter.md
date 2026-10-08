# TF-024: OMP local worker adapter

## Goal

Launch one task through OMP using the v1 launch and result contract, selecting
`ornith1.5-35B` explicitly with no fallback, against a project-local oMLX
endpoint, recording the run as `exit`, `blocked`, or `stalled` and running
`bin/test` and `bin/lint` in the claimed worktree.

## Dependencies

- TF-010
- TF-011

## Scope

Implement the OMP adapter that maps the launch contract to observed OMP v18.8.5
flags: `--model=ornith1.5-35B` (explicit model), `--cwd <worktree>`,
non-interactive `-p/--print`, observable progress via `--mode rpc`, and a
wall-clock bound via `--max-time`. Run a preflight that refuses an unavailable
model or endpoint before launch. Record the result commit, changed files, and
evidence path.

## Constraints

Do not fall back to `--smol`, `--plan`, `--slow`, Gemma, or any default model;
the example selects `ornith1.5-35B` explicitly. Do not launch a model during
verification — only read `omp --help` and confirm the required flags exist.
Store no secrets in task or evidence files.

## Success criteria

### C1: The OMP launch example selects ornith1.5-35B explicitly with no fallback

Check: cat docs/local-workers-v1.md

### C2: The preflight refuses an unavailable model or endpoint before launch

Check: go test ./...

### C3: The worker runs bin/test and bin/lint in the worktree and records each exit code

Check: bin/test

### C4: The repository checks pass

Check: bin/test

## Verification

Compare the documented OMP invocations with `omp --help` without launching a
model; confirm `--model`, `--cwd`, `-p/--print`, `--mode rpc`, and `--max-time`
are present. Run `bin/test`, `bin/lint`, and the whole-tree validator; report
exact exits and any unverified adapter assumption.

# TF-025: Pi local worker adapter

## Goal

Launch one task through Pi using the v1 launch and result contract, selecting a
model explicitly with no fallback, against a project-local oMLX endpoint,
recording the run as `exit`, `blocked`, or `stalled`, and running `bin/test` and
`bin/lint` in the claimed worktree.

## Dependencies

- TF-010
- TF-011

## Scope

Implement the Pi adapter that maps the launch contract to observed Pi flags:
`--model <id>` pinned to an explicit id (a `provider/id` pattern), `--provider`,
non-interactive `-p/--print`, observable progress via `--mode rpc`, and manual
permissions (do not pass `-a`/`--approve`). Run a preflight that refuses an
unavailable model or endpoint before launch. Record the result commit, changed
files, and evidence path.

## Constraints

Pin the model explicitly; do not rely on a default or let Pi pick another model.
Do not launch a model during verification — only read `pi --help` and confirm
the required flags exist. No timeout flag was observed in `pi --help`; treat any
wall clock bound as planned and record it as such. Store no secrets in task or
evidence files.

## Success criteria

### C1: The Pi launch example selects a model explicitly with no fallback

Check: cat docs/local-workers-v1.md

### C2: The preflight refuses an unavailable model or endpoint before launch

Check: go test ./...

### C3: The worker runs bin/test and bin/lint in the worktree and records each exit code

Check: bin/test

### C4: The repository checks pass

Check: bin/test

## Verification

Compare the documented Pi invocations with `pi --help` without launching a
model; confirm `--model`, `--provider`, `-p/--print`, and `--mode rpc` are
present. Run `bin/test`, `bin/lint`, and the whole-tree validator; report exact
exits and any unverified adapter assumption.

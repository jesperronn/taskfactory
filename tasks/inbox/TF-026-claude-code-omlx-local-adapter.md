# TF-026: Claude Code adapter through the oMLX local endpoint

## Goal

Launch one task through Claude Code routed through a project-local oMLX server,
selecting `ornith1.5-35B` explicitly with no fallback, and resolve the TF-015
outcome where Claude Code "warned that the Ornith model was not in its local
model catalog" by ensuring the model is in the local catalog before launch.

## Dependencies

- TF-010
- TF-011
- TF-015

## Scope

Implement the Claude Code adapter that maps the launch contract to observed
flags against a local oMLX endpoint: start
`omlx serve --host 127.0.0.1 --port 8000` and launch Claude Code with
`omlx launch claude --host 127.0.0.1 --port 8000`, invoking Claude Code directly
with `--model ornith1.5-35B --add-dir <worktree>`, non-interactive `-p/--print`,
observable progress via `--output-format stream-json`, and manual permissions
via `--permission-mode manual`. Run a preflight that refuses an unavailable
model or endpoint before launch, and do not pass `--fallback-model`.

## Constraints

Select `ornith1.5-35B` explicitly; do not pass `--fallback-model`. Resolve the
TF-015 warning by confirming the model is present in the oMLX local catalog
before launch and refusing otherwise. Do not launch a model during verification
— only read `claude --help` and the oMLX help, and confirm the required flags
exist. A wall-clock timeout is planned here (Claude Code exposes
`--max-budget-usd`, a cost cap, not a time bound); record it as such. Store no
secrets in task or evidence files.

## Success criteria

### C1: The Claude Code launch routes through the local oMLX endpoint and selects ornith1.5-35B with no fallback

Check: cat docs/local-workers-v1.md

### C2: The preflight resolves the TF-015 "model not in catalog" warning and refuses before launch

Check: go test ./...

### C3: The worker runs bin/test and bin/lint in the worktree and records each exit code

Check: bin/test

### C4: The repository checks pass

Check: bin/test

## Verification

Compare the documented oMLX and Claude Code invocations with
`omlx serve --help`, `omlx launch --help`, and `claude --help` without launching
a model; confirm the endpoint flags, `--model`, `--output-format stream-json`,
and `--permission-mode manual` are present. Run `bin/test`, `bin/lint`, and the
whole-tree validator; report exact exits and any unverified adapter assumption.

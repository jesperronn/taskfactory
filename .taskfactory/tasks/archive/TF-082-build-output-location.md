# TF-082: Build into dist/ and empty it first

## Goal

The default `bin/build` output moves from `bin/out/taskfactory` to
`dist/taskfactory`, the owner's preferred build directory, which release
packaging also uses. The Go default is `go build` writing `./taskfactory` in
the current directory, which clutters the repository root, and `go install`
writing to GOBIN, which `bin/install` already covers. Because `dist/` is also
where release archives are produced, a default build empties it first so stale
artifacts never mix with fresh ones.

## Dependencies

None

## Scope

Decisions are final:

1. `bin/build` default output is `dist/taskfactory`. `bin/build
[output-path]` still overrides it.
2. Before a build whose output path is under the repository's `dist/`
directory (the default, or an explicit path inside it), `bin/build` removes
the contents of `dist/` but never the `dist/` folder itself. An explicit
output path elsewhere never triggers any removal. The removal is guarded: the
directory variable must be non-empty, must resolve to `<repo root>/dist`, and
the `rm` uses that absolute path with a trailing `/*`-style glob or `find
-mindepth 1`, never a bare variable that could be empty.
3. Release scripts (TF-072 and later) must also empty `dist/` before producing
archives; TF-072 gets a one-line note saying so.
4. `.gitignore` lists `dist/` and no longer lists `bin/out/`.
5. `bin/install` passes an explicit target outside `dist/`, so it needs no
behaviour change; check that `bin/install.test.sh` still passes.
6. `bin/build.test.sh` gains a scenario `output`, in the scenario-per-argument
style of TF-081, that checks the default path is `dist/taskfactory`, that a
stale file in `dist/` is removed by a default build, that an explicit path
outside `dist/` is honored and leaves `dist/` contents alone, and that
`.gitignore` lists `dist/` and not `bin/out/`. Existing tests that reference
`bin/out/taskfactory` are updated.
7. `README.md` gets a short paragraph in the `bin/build` description naming
`dist/taskfactory` as the default, saying that a default build empties `dist/`,
and that `bin/install` puts the binary on PATH. Update
`skills/taskfactory/SKILL.md` or `docs/release.md` only if they name
`bin/out`; at the time of writing they do not.

## Constraints

Run this task only after TF-081 is archived: both change `bin/build` and
`bin/build.test.sh`. The validator only accepts archived dependencies, so
the order is stated here instead of under Dependencies.

Touch only `bin/build`, `bin/build.test.sh`, `.gitignore`, `README.md`,
`tasks/inbox/TF-072-release-packaging.md` (one line) and, only if needed,
`bin/install.test.sh`. No Go changes, no new dependencies.

## Success criteria

### C1: The default path, cleaning and explicit path are tested

Check: bin/build.test.sh output

### C2: The ignore file lists dist/ and not bin/out/

Check: grep -qx 'dist/' .gitignore && ! grep -q 'bin/out' .gitignore

### C3: The README names the default output path

Check: grep -q 'dist/taskfactory' README.md

### C4: The full test suite passes

Check: bin/test

## Verification

Run `bin/build.test.sh output`, `bin/test`, `bin/lint` and
`go run ./cmd/taskfactory validate tasks` from the repository root and report
each exit code.

## Claim

Owner: sonnet-worker-082
Branch: task/TF-082
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-082
Base commit: 8a61ce51d8251750d46e7f0084685c29b72c6012
Started at: 2026-10-20T23:09:03Z

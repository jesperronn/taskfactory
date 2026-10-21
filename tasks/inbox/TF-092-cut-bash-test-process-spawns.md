# TF-092: Cut process spawns in bash tests

## Goal

Every bash test finishes in under 1 second, measured with `--verbose`, by
stopping per-test `go build`, `go run` and tool startups where a test does not
need the real tool.

## Dependencies

- TF-087

## Scope

Bash test wall times before the loop, from the baseline in TF-087:
`release.test.sh` 26s, `lint.test.sh` 17s, `build.test.sh` 13s,
`package.test.sh` 13s, `install.test.sh` 11s, `release.version.test.sh` 5.5s,
`release-publish.test.sh` 5.1s, `lint.task.test.sh` 4.5s,
`lint.real-mdsmith.test.sh` 2.9s.

Main causes:

- `bin/build.test.sh` and `bin/install.test.sh` run real `go build` for each
  scenario (`bin/build.test.sh` has 11 call sites, `bin/install.test.sh` 2).
- `bin/lint` runs `go run github.com/jeduden/mdsmith/cmd/mdsmith@v0.57.0`
  (`bin/lint:52`), so every lint test pays for the mdsmith build. Tests that
  check the lint wrapper's logic should use a stub, as `bin/lint.test.sh` already
  does with `LINT_STUB_EXIT`. Keep the real-mdsmith tests as one end-to-end case
  per file.
- `shellcheck` runs over every Bash file on each `bin/lint` call.
- `bin/release.test.sh` and `bin/package.test.sh` build real artifacts. Keep
  one real build per file and stub the rest.

Use `--verbose` (TF-087) to see each case's time, and make the slowest case
the first target.

## Constraints

- Keep one real end-to-end check for each tool the wrapper calls.
- Do not remove a `[PASS]` assertion. A stubbed check must still assert what the
  wrapper passes to the tool.
- Run `bin/lint` on the touched files and `bin/test` before reporting.

## Success criteria

### C1: Every bash test is under a second

Check: bin/check-speed bash

### C2: The full bash suite still passes

Check: bin/test

## Verification

Run `bin/test --verbose` and report each file's per-case timing and the exit
code. Run `bin/check-speed bash` and report its exit code.

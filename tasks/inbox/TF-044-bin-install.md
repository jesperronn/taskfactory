# TF-044: Add bin/install

## Goal

Give users one command that installs the `taskfactory` binary with the same
embedded version as `bin/build`, so the `-ldflags` detail is never retyped.

## Dependencies

- TF-031

## Scope

Add `bin/install`, a short Bash wrapper around `bin/build`. It installs to
`$PREFIX` when set, otherwise `$GOBIN` when set, otherwise `$(go env GOPATH)/bin`,
creating the directory if needed, and builds straight to
`<destination>/taskfactory` through `bin/build <path>` so the version logic
lives in one place. It accepts no arguments (usage error exit 2 for any) and
prints the installed path and version. Add `bin/install.test.sh` in the style of
`bin/build.test.sh`, using a temporary `PREFIX`. Mention it in the README
install section and in `docs/ci.md` only if that file lists build commands. The
release script (TF-032) can reuse it later; do not implement release here.

## Constraints

Do not duplicate the `git describe` or `-ldflags` logic from `bin/build`, do not
write outside the chosen destination, and do not require network access beyond
Go's normal module handling. Never modify the user's shell profile or `PATH`;
at most print a hint when the destination is not on `PATH`.

## Success criteria

### C1: Install writes an executable binary to PREFIX

Check: bin/install.test.sh

### C2: The installed binary reports the embedded version

Check: PREFIX="$(mktemp -d)" bin/install && "$PREFIX/taskfactory" --version

### C3: Arguments are rejected with exit 2

Check: bin/install extra; test $? -eq 2

### C4: Repository checks pass

Check: bin/test

## Verification

Run the four checks, `bin/lint` and `go run ./cmd/taskfactory validate`. Report
each exit code, and show that `bin/install` with no `PREFIX` or `GOBIN` set
targets `$(go env GOPATH)/bin` (use `bin/install.test.sh` or a dry run with
`GOPATH` pointed at a temporary directory; do not overwrite a real installed
binary).

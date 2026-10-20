# TF-086: Make bin/lint ignore claimed task worktrees

## Goal

`bin/lint` on the main checkout passes while tasks are claimed. Claimed task
worktrees live under `.taskfactory/worktrees/<ID>/` and are full copies of the
repository. Today mdsmith lints their misformatted fixtures, and the Go and
shell walks of `bin/lint` can pick up their scripts, so a whole-project run
fails whenever any task is claimed.

## Dependencies

- TF-030
- TF-038

## Scope

Edit `.mdsmith.yml` and `bin/lint`, and extend
`bin/lint.real-mdsmith.test.sh`. Decisions are final:

1. Add `.taskfactory/**` to the `ignore:` list in `.mdsmith.yml`. Only
`.taskfactory/config.toml` is tracked there and it is TOML, so no Markdown
is lost.
2. Prune `.taskfactory` in both `find` expressions of the whole-project walk
in `bin/lint`. Every other behaviour stays as it is.
3. A path given explicitly on the command line, inside `.taskfactory/` or
anywhere else, is linted as asked. Only the whole-project run skips the tree.
4. The regression test builds a fixture project with bad Markdown, an
unformatted Go file and a shellcheck-failing script under
`.taskfactory/worktrees/TF-999/`, and clean content elsewhere.

## Constraints

Change no flag, exit code or message of `bin/lint`. Keep scripts clean under
shellcheck. The ignore must not be broader: the same bad files outside
`.taskfactory/` must still fail a whole-project run.

## Success criteria

### C1: The mdsmith ignore list names the runtime tree

Check: grep -q '^  - \.taskfactory/\*\*$' .mdsmith.yml

### C2: Both walks in bin/lint prune .taskfactory

Check: test "$(grep -c 'name \.taskfactory' bin/lint)" -ge 2

### C3: The regression test passes

Check: bin/lint.real-mdsmith.test.sh

### C4: The whole project lints clean

Check: bin/lint

### C5: The full test suite passes

Check: bin/test

### C6: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each Check command, `bin/test`, `bin/lint` and
`go run ./cmd/taskfactory validate tasks` from the repository root and report
each exit code. Also show `bin/lint` failing on the base commit and passing
with the change when a copy of the fixtures sits under a throwaway
`.taskfactory/worktrees/` directory.

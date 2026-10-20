# TF-084: Print a next-step hint after every command

## Goal

Every `taskfactory` subcommand ends with a short, consistent hint about what
to do next, so a new user can follow the lifecycle from `init` to
`integrate` without reading the README. `init` prints a "Next steps" block, the
other commands print one line, and failures that have an obvious recovery
print a hint on stderr.

## Dependencies

None

## Scope

Edit `cmd/taskfactory/main.go`, add one shared helper file
`cmd/taskfactory/nextstep.go`, and add `cmd/taskfactory/nextstep_test.go`.
Update the README command reference and `skills/taskfactory/SKILL.md`.
Decisions are final:

1. One table in `nextstep.go` maps a command and an outcome to its hint text.
Every subcommand obtains its hint from the helper functions
`nextOnSuccess(p, command, id)` and `nextOnFailure(p, command, id)`; no command
writes a hint string of its own. `work` already prints `next:` lines from
`internal/work`; they stay as they are and must read identically to the table
below (the table is the single source and `work` keeps its own code path, which
a test pins).
2. Placement. A success hint goes to stdout after the normal output. A failure
hint goes to stderr after the existing error line. The machine-readable output
(the `status` counts, the `verify` summary line, `integrated <ID>`,
`check-main: ok`, `claimed ...`, `failed ...`, `promoted ...`, `requeued ...`)
is unchanged and stays first. Exit codes do not change. Usage errors (exit 2)
print no hint.
3. A hint is `next: ` followed by a command in plain text, in the same style
`work` uses today. `init` prints this block instead of one line, after
its normal output:

   - the line `Next steps:`
   - `  1. Review .taskfactory/config.toml and set the verification commands.`
   - `  2. Commit .taskfactory/config.toml, .gitignore and tasks/ first.`
   - `  3. Write a task in tasks/inbox, then run: taskfactory promote <ID>`
   - `  4. Run: taskfactory claim <ID> --owner <name>`

   Running `init` a second time on an initialized project prints the same block.
4. Exact hints; `<ID>` is the task ID given, `<N>` the owner given.

- `init`
  success: the Next steps block above
  failure: none
- `status`
  success: `next: taskfactory validate`
  failure: `next: run taskfactory init at the repo root`
- `validate`
  success: `next: taskfactory promote <ID> or claim <ID>`
  failure: `next: fix the files, rerun taskfactory validate`
- `promote`
  success: `next: taskfactory claim <ID> --owner <name>`
  failure: `next: fix tasks/inbox, rerun taskfactory promote <ID>`
- `claim`
  success: `next: taskfactory work <ID> --adapter <name> --model <id>`
  failure: `next: taskfactory status shows the task state`
- `work`
  success: unchanged, see Scope 1
  failure: unchanged
- `verify`
  success: on PASS `next: taskfactory integrate <ID>`
  failure: on FAIL `next: fix the checks, rerun taskfactory verify <ID>`
- `integrate`
  success: `next: taskfactory status`
  failure: `next: rerun taskfactory verify <ID>, then integrate <ID>`
- `check-main`
  success: `next: taskfactory integrate <ID>`
  failure: `next: fix main, rerun taskfactory check-main`
- `fail`
  success: `next: taskfactory requeue <ID> --to ready`
  failure: `next: taskfactory status shows the task state`
- `requeue`
  success: `next: taskfactory claim <ID> --owner <name>`
  failure: `next: taskfactory status shows the task state`

   Placeholders stay literal and `<ID>` is the real task ID. A success of
   `requeue --to inbox` prints `next: taskfactory promote <ID>` instead.

5. Help. Each command's help constant gets one line after the description and
before `Flags:` that begins `Next step:` and names the success hint in prose,
for example `Next step: taskfactory integrate <ID> after a passing verify.`
The top-level usage text gets no per-command hint. `status` and `validate`, as
info commands, give the single line from the table; they never print the
multi-line block.
6. Styling. The helper takes a `ui.Painter` for the stream it writes to,
colors only the word `next:` (dim), and with color off the output is
byte-identical to the table.

## Constraints

Change no machine-readable output line, no exit code, no flag and no behaviour
inside `internal/*` except what Scope 1 requires (none expected). Do not add
flags; `--verbose` and `--dry-run` belong to TF-085. If TF-083 (init with a
path) is not in main when this task starts, `init` takes no argument and the
Next steps block is printed after the existing behaviour; if it has merged,
the block names the target directory in step 2. Follow the existing
`errorLine` style for stderr. Tests use temporary Git repositories only.

## Success criteria

### C1: init prints the Next steps block on stdout

Check: go test ./cmd/taskfactory -run 'TestNextStepInitBlock$'

### C2: A second init prints the same block

Check: go test ./cmd/taskfactory -run 'TestNextStepInitRerun$'

### C3: status and validate print exactly one next line each

Check: go test ./cmd/taskfactory -run 'TestNextStepInfoCommandsOneLine$'

### C4: promote, claim, fail and requeue print their success hints

Check: go test ./cmd/taskfactory -run 'TestNextStepLifecycleSuccess$'

### C5: verify prints the PASS hint after the summary line

Check: go test ./cmd/taskfactory -run 'TestNextStepVerifyPass$'

### C6: verify prints the FAIL hint on stderr and still exits 1

Check: go test ./cmd/taskfactory -run 'TestNextStepVerifyFail$'

### C7: integrate and check-main print their success and failure hints

Check: go test ./cmd/taskfactory -run 'TestNextStepIntegrateAndCheckMain$'

### C8: fail failure and requeue failure print a status hint on stderr

Check: go test ./cmd/taskfactory -run 'TestNextStepFailureHintsOnStderr$'

### C9: Usage errors print no hint and exit 2

Check: go test ./cmd/taskfactory -run 'TestNextStepUsageErrorsHaveNoHint$'

### C10: Machine-readable lines and exit codes are unchanged

Check: go test ./cmd/taskfactory -run 'TestNextStepMachineOutputUnchanged$'

### C11: Every command's help has a Next step line

Check: go test ./cmd/taskfactory -run 'TestNextStepHelpLinePerCommand$'

### C12: Hints come from the shared helper and work keeps identical text

Check: go test ./cmd/taskfactory -run 'TestNextStepTableIsSingleSource$'

### C13: With color off the hint is plain text

Check: go test ./cmd/taskfactory -run 'TestNextStepPlainWithoutColor$'

### C14: The README documents next-step hints

Check: grep -q 'Next steps' README.md

### C15: The full test suite passes

Check: bin/test

### C16: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each Check command, `bin/test`, `bin/lint` and
`go run ./cmd/taskfactory validate tasks` from the repository root and report
each exit code. Also run `go run ./cmd/taskfactory init` in a scratch Git
repository and paste the printed Next steps block.

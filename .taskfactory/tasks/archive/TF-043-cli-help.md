# TF-043: Describe commands and add per-subcommand help

## Goal

Make `taskfactory --help` self-explanatory and let users learn a command with
`taskfactory <command> --help`.

## Dependencies

- TF-001

## Scope

Three changes, committed separately in this order.

1. Top-level usage text. List every real command, including `init`, which the
   code handles but the usage omits. Give each command line a 3 to 5 word
   description, aligned in one column. Document only `--help` and `--version`
   as global flags; the single-dash spellings keep working silently because the
   standard `flag` package accepts them, but they are no longer shown. Example
   shape: `status` shows tasks by state, `claim <ID> --owner <name>` claims a
   ready task, `integrate <ID>` fast-forwards a verified task, `check-main`
   rechecks a stopped main.
2. Per-command help. `taskfactory <command> --help` (and `-h`) prints, to stdout
   with exit 0, a usage line, a one-paragraph description, the command's flags
   and its exit codes, for `init`, `status`, `validate`, `claim`, `verify`,
   `integrate` and `check-main`. `--help` after a command wins over any other
   arguments and does not require project config or touch any state. Keep each
   help text as a constant next to its command.
3. Tests and docs. Add one test per command asserting exit 0, stdout, and the
   usage line, plus a test that every command in the top-level list has help.
   Update `docs/technical-spec-v1.md` and the README only where they quote the
   usage text.

Defer a man page; it can be generated from this help text later.

## Constraints

Invalid commands and invalid flags still print to stderr with exit 2. Do not
change command behavior, exit codes or output other than help text. Color
behavior from TF-042 must continue to apply: plain text when piped or when
`NO_COLOR` is set, and byte-identical to the new plain text. Update existing
usage-text tests to the new text without weakening what they assert. No new
dependencies.

## Success criteria

### C1: Top-level help lists init and describes every command

Check: go run ./cmd/taskfactory --help

### C2: Every subcommand has help that exits 0

Check: go test ./cmd/taskfactory

### C3: Help works outside a project and changes nothing

Check: go test ./cmd/taskfactory -run Help

### C4: Repository checks pass

Check: bin/test

## Verification

Run the four checks, `bin/lint` and `go run ./cmd/taskfactory validate`. Also
run `--help` for each of the seven commands piped through `cat -v` and confirm
no escape codes, and once in a real terminal to confirm the aligned column.
Report each exit code.

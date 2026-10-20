# TF-085: Add --verbose and --dry-run to every subcommand

## Goal

Every `taskfactory` subcommand accepts `--verbose`, which adds detail on
stderr, and `--dry-run`, which prints what would change and changes nothing:
no file move, no commit, no worktree, no Git ref change, no evidence write and
no adapter launch. Both flags are parsed in one shared place so they behave the
same everywhere.

## Dependencies

- TF-084

## Scope

Add `cmd/taskfactory/commonflags.go` and `cmd/taskfactory/dryrun_test.go`, and
edit `cmd/taskfactory/main.go`, the argument parsers in `internal/work`,
`internal/fail`, `internal/requeue` and `internal/promote` only as far as
needed to let them skip flags the shared parser has removed, and the mutating
packages `internal/claim`, `internal/promote`, `internal/fail`,
`internal/requeue`, `internal/verify`, `internal/integrate` and `internal/work`
to accept an `Options.DryRun` and `Options.Verbose`. Update the README and
`skills/taskfactory/SKILL.md`. Decisions are final:

1. Shared parsing. `commonflags.go` has one function that removes `--verbose`
and `--dry-run` from the argument list of any subcommand and returns the
remaining arguments with the two booleans. The flags are accepted before or
after the positionals and between flags, for example
`taskfactory --dry-run promote TF-1` is not accepted (global flags stay
`--help` and `--version`) but `promote --dry-run TF-1` and
`promote TF-1 --dry-run` both are. A flag given twice is accepted once. A
value form such as `--dry-run=true` is a usage error. Each subcommand's own
parser sees only the remaining arguments, so existing error messages and
exit-2 cases are unchanged. Every command's help lists both flags in `Flags:`.
2. `--verbose` writes lines starting `verbose: ` to stderr only: the Git
commands run (as `verbose: git <args>`), the paths read or written, the
project root and the task paths touched. It never changes stdout and never
changes an exit code. Secrets and environment values are redacted the way
`work` already redacts its output.
3. `--dry-run` prints each action on stdout as a line starting `would: ` and
ends with the line `dry run: nothing changed`. It validates its inputs and
refuses exactly as the real command would for a usage error (exit 2) or an
invalid or missing task (exit 1), because a dry run that reports success for
something that would fail is misleading. Exit 0 means the real command would
likely proceed. It never writes the TF-084 next-step hint; instead the final
line is `dry run: nothing changed`.
4. Per command; "no-op" means accepted, documented in help, nothing printed
except the normal output followed by `dry run: nothing changed`, exit 0.

- `init`:
  `would: create <path>` per missing config, directory, ignore line;
  creates nothing
- `status`: no-op, it only reads
- `validate`: no-op, it only reads
- `check-main`:
  `would: run <command>` per integration check;
  runs none, reads only
- `promote`:
  `would: move task to tasks/ready`, `would: commit`;
  moves and commits nothing
- `claim`:
  `would: move task to tasks/active`, `would: add worktree <path>`;
  creates no branch or worktree
- `fail`: `would: move task to tasks/failed`, `would: commit`; changes nothing
- `requeue`:
  `would:` move, commit and worktree and branch removal;
  changes nothing
- `verify`:
  `would: run <check>` per check, `would: append evidence`;
  runs nothing, writes no evidence
- `integrate`:
  `would: run <command>`, `would: fast-forward main`, `would: archive`;
  changes nothing
- `work`:
  `would: launch <adapter>` and `would: write log`;
  never launches, writes no log

5. Dry-run implementation rule: the dry-run branch runs the same validation
code as the real command and stops before the first write; it is not a copy of
the checks. A shared `dryrun` output helper in `commonflags.go` prints the
`would: ` lines and the closing line so wording is consistent.
6. Exit codes: dry run follows the real command: 0, 1 and 2 as documented. The
help text of each command gains the two flags and the sentence `With --dry-run
nothing changes.` The info commands say `--dry-run is accepted and has no
effect, because this command only reads.`
7. Docs. The README documents both flags with one example each, and the skill
tells agents to use `--dry-run` before `claim`, `integrate` and `requeue`
when unsure.

## Constraints

Depends on TF-084, which edits the same help constants and output code; do
not start before it is merged. If TF-083 (init with a path) is not in main
when this task starts, `init --dry-run` takes no path; if it has merged,
`--dry-run` applies to the path form too. At the time of writing TF-083 is not
in main, so it is not a dependency. Do not change any stdout line of a real
(non dry-run) run, any exit code or any adapter behaviour. A dry run must not
create `.taskfactory/` files other than none: not the lock files, not logs, not
evidence. Tests use temporary Git repositories only.

## Success criteria

### C1: Both flags parse before and after positionals

Check: go test ./cmd/taskfactory -run 'TestCommonFlagsPlacement$'

### C2: A repeated flag is accepted and a value form is a usage error

Check: go test ./cmd/taskfactory -run 'TestCommonFlagsRepeatAndValueForm$'

### C3: Every command help lists --verbose and --dry-run

Check: go test ./cmd/taskfactory -run 'TestHelpListsVerboseAndDryRun$'

### C4: Info commands accept --dry-run as a documented no-op

Check: go test ./cmd/taskfactory -run 'TestDryRunInfoCommandsNoOp$'

### C5: A dry run of init, promote and claim changes nothing

Check: go test ./cmd/taskfactory -run 'TestDryRunInitPromoteClaimChangeNothing$'

### C6: A dry run of fail and requeue changes nothing

Check: go test ./cmd/taskfactory -run 'TestDryRunFailRequeueChangeNothing$'

### C7: A dry run of verify runs no check and writes no evidence

Check: go test ./cmd/taskfactory -run 'TestDryRunVerifyWritesNoEvidence$'

### C8: A dry run of integrate and check-main changes nothing

Check: go test ./cmd/taskfactory -run 'TestDryRunIntegrateCheckMainChangeNothing$'

### C9: A dry run of work never launches the adapter or writes a log

Check: go test ./cmd/taskfactory -run 'TestDryRunWorkNeverLaunchesAdapter$'

### C10: Git status and git worktree list are identical before and after every mutating dry run

Check: go test ./cmd/taskfactory -run 'TestDryRunLeavesGitStatusAndWorktreesUnchanged$'

### C11: Dry-run output uses would lines and the closing line

Check: go test ./cmd/taskfactory -run 'TestDryRunOutputFormat$'

### C12: A dry run refuses what the real command refuses, with the same exit code

Check: go test ./cmd/taskfactory -run 'TestDryRunRefusesLikeRealCommand$'

### C13: Verbose writes verbose lines to stderr only and leaves stdout and exit code unchanged

Check: go test ./cmd/taskfactory -run 'TestVerboseStderrOnly$'

### C14: Verbose redacts secrets

Check: go test ./cmd/taskfactory -run 'TestVerboseRedactsSecrets$'

### C15: Existing usage errors still exit 2 with unchanged messages

Check: go test ./cmd/taskfactory -run 'TestFlagsKeepExistingUsageErrors$'

### C16: The README documents --dry-run

Check: grep -q -e '--dry-run' README.md

### C17: The full test suite passes

Check: bin/test

### C18: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each Check command, `bin/test`, `bin/lint` and
`go run ./cmd/taskfactory validate tasks` from the repository root and report
each exit code. Also run `git status --short` and `git worktree list` before
and after a `--dry-run` of `claim` in a scratch repository and paste both
outputs.

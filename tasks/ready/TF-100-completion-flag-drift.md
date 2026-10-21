# TF-100: Complete every real flag and make the drift test bidirectional

## Goal

Tab-completion offers every real flag of every command, and a test fails when
a command gains a flag that has no completion entry, so the table cannot drift
from the code in either direction.

## Dependencies

None

## Scope

Edit `cmd/taskfactory/completion.go` and `completion_test.go`. The true flag
set, read from the parsers (`internal/work/args.go`, `internal/fail/fail.go`,
`internal/requeue/requeue.go`, the claim flag set in `main.go`) and the
`Flags:` section of each help text, is:

- init: none of its own
- status: none of its own
- validate: none of its own
- promote: none of its own
- claim: `--owner` (free value)
- work: `--adapter`, `--model`, `--haiku-model`, `--endpoint`, `--timeout`
- verify: none of its own
- integrate: none of its own
- check-main: none of its own
- fail: `--outcome`, `--reason` (free value)
- requeue: `--to`, `--discard-claim`
- completion: none of its own
- every command except completion: `--verbose`, `--dry-run`
- every command: `--help` (help also prints `-h`, a short alias)

Decisions are final:

1. Add to `completionSpecs`: `fail --reason` and `work --haiku-model` and
`work --endpoint`, each a free value, so bash and zsh offer them and skip
their value word.
2. The bidirectional test derives each command's real flag set from the
`Flags:` section of `commandHelps[command]`, the least brittle source because
every command already prints that section and a new flag is documented there
first. It reads the lines after `Flags:` up to the next blank line, takes the
first `--name` token of each line and ignores `-h`. No second hand-kept list
of flags is added to the test.
3. The test then requires equality: every flag in the help `Flags:` section
has a table entry, or is a common flag, or is `--help`, or is in the exempt
set; and every table flag appears in that section. A flag in either set only
fails the test with a message naming the command and the flag.
4. The exempt set is one variable in `completion.go`,
`completionHiddenFlags`, a map from command to flag names that are real but
deliberately not completed. It starts empty. The `__complete` helper is a
hidden command, not a flag, and stays as it is.
5. The test also fails if a command's help has no `Flags:` section, so a new
command cannot slip through unchecked, and if an exempt entry names a flag
that is not in the help.
6. `internal/work` and `internal/fail` parsers stay unchanged. The README
lists no per-command flag table for completion, so it is edited only if a
check below shows otherwise.

## Constraints

Do not change the command behavior, the help texts, the bash or zsh script
templates or the existing bash and zsh output tests. Do not weaken
`TestCompletionTableMatchesCommands`. A task that adds a command, such as the
`skill` command of TF-095, must add its completion entry and help `Flags:`
section; that is the intended effect of this test and not a regression here.

## Success criteria

### C1: Every help flag is in the table or exempt

Check: go test ./cmd/taskfactory -run TestCompletionFlagsCoverHelp

### C2: Every table flag is in the command help

Check: go test ./cmd/taskfactory -run TestCompletionTableFlagsInHelp

### C3: The drift test fails on a flag missing from the table

Check: go test ./cmd/taskfactory -run TestCompletionDriftDetectsMissingFlag

### C4: The exempt set is checked and lives in one place

Check: go test ./cmd/taskfactory -run TestCompletionHiddenFlagsExempt

### C5: Bash completes the newly added flags

Check: go test ./cmd/taskfactory -run TestBashCompletionAddedFlags

### C6: The existing bash and zsh output tests stay green

Check: go test ./cmd/taskfactory -run 'TestBashCompletion$|TestZshCompletionSyntax'

### C7: The three flags are in the table source

Check: grep -c -e '--reason' -e '--haiku-model' -e '--endpoint' cmd/taskfactory/completion.go

### C8: The exempt set is declared in completion.go

Check: grep -q 'var completionHiddenFlags' cmd/taskfactory/completion.go

### C9: The full test suite passes

Check: bin/test

### C10: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each named test above, `bin/test`, `bin/lint` and
`go run ./cmd/taskfactory validate tasks` from the repository root and report
each exit code. In a scratch edit only, remove `--endpoint` from
`completionSpecs`, show that `TestCompletionFlagsCoverHelp` fails naming
`work` and `--endpoint`, then revert the edit. Report the grep output of C7.

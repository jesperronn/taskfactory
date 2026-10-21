# TF-088: Bash and zsh tab-completion for taskfactory and tf

## Goal

`taskfactory completion bash|zsh` prints a completion script that completes
subcommands, flags, flag values and task IDs for both `taskfactory` and `tf`,
generated from one table that also drives the command set, so completion
cannot drift from the commands.

## Dependencies

- TF-084
- TF-085

## Scope

Add `cmd/taskfactory/completion.go` and tests, extend
`cmd/taskfactory/main.go` and `nextstep.go`, the README and
`skills/taskfactory/SKILL.md`, and make a small hint change in `bin/install`
and `bin/install.test.sh`. Decisions are final:

1. Command `taskfactory completion <bash|zsh>` prints the script on stdout and
exits 0. No shell, a third shell name or extra arguments is a usage error on
stderr in the usual `taskfactory completion: ...` form and exits 2.
`completion --help` prints a `completionHelp` text in the existing help style,
registered in the command table and the top-level command list like every
other command.
2. One table, `completionSpecs` in `completion.go`, has one entry per command
with its positional kind and its flags. A positional kind is one of `none`,
`ids:<state>`, `path` or `paths`. A flag has a name, a value kind (`none`, a
fixed list or `free`) and the fixed values. The table holds: init (path),
status, validate (paths), promote (ids:inbox), claim (ids:ready, `--owner`
free), work (ids:active, `--adapter` omp|pi|claude, `--model` free,
`--timeout` free), verify (ids:active), integrate (ids:active verified),
check-main, fail (ids:active, `--outcome` FAILED|BLOCKED), requeue
(ids:failed, `--to` ready|inbox, `--discard-claim`), completion (bash zsh).
The common flags `--verbose` and `--dry-run` are offered for every command
except completion, and `--help` for every command. Top level offers the
commands plus `--help` and `--version`. The command list is derived from the
existing command table, not retyped.
3. The shell scripts are Go text templates rendered from that table. They
contain no hand-listed command or flag outside the table. Both register the
completion for `taskfactory` and `tf`: bash with
`complete -F _taskfactory taskfactory tf`, zsh with
`compdef _taskfactory taskfactory tf`. The zsh script works when sourced with
`source <(taskfactory completion zsh)` and when saved as `_taskfactory` on
`fpath` (it then calls `compdef` only when sourced). Neither script needs
`bash-completion` or any external tool.
4. Dynamic values come from a hidden helper,
`taskfactory __complete <kind> <command> [prefix]`. It is not listed in usage
or in `--help`, is not in the command table, and prints one candidate per
line. For `ids` it lists the task IDs, taken from the file names
`TF-NNN-*.md`, of the state directory the command takes: promote inbox, claim
ready, work, verify and fail active, integrate active tasks that have
verification evidence, requeue failed. It finds the project root the way other
commands do, from any cwd inside the repo, reads only directory names, runs no
git and no subprocess, and exits 0 with no output and nothing on stderr
outside a repository, for an unknown kind or command, or for a missing
directory. The prefix filters by `HasPrefix`. Output is sorted.
5. Flag-value completion for `--adapter`, `--outcome` and `--to` uses the
fixed lists in the table. `--model`, `--owner` and `--timeout` offer nothing.
`path` and `paths` fall back to the shell's file completion. Already-given
flags are not offered again, `--` ends flag completion, and a word starting
with `-` is completed from the flag list of the command.
6. `__complete` and `completion` print no next-step hint and are exempt from
the common flags. Add a `completion` entry to the TF-084 hint table whose
success text reads `add it to your shell profile, see README` and whose
failure text reads
`taskfactory completion bash or taskfactory completion zsh`. `completion`
prints the script only, with the hint going to stderr only when stdout is a
terminal, so `source <(...)` stays clean.
7. `bin/install` does not edit dotfiles and does not install completion files.
After installing it prints one extra stdout hint line,
`hint: enable tab completion with: source <(taskfactory completion bash)` (see
README for zsh). `TASKFACTORY_NO_ALIAS` does not affect it. Update the
`default` scenario of `bin/install.test.sh` to expect the line.
8. README gets a Shell completion section with the bash line
`source <(taskfactory completion bash)` for `~/.bashrc`, the zsh line
`source <(taskfactory completion zsh)` for `~/.zshrc` placed after `compdef`
is available (`autoload -Uz compinit && compinit`), and the fpath alternative.
It says `tf` completes too, that task IDs are read from the task directories
of the repository you are in, and that nothing is offered outside a
repository. The skill gets a short note that completion exists and is
humans-only.
9. Tests, all Go, in `cmd/taskfactory/completion_test.go` using temporary
directories and a fixture repository created with `git init` and
`git config commit.gpgsign false`. They cover the sections below and use the
CLI built once per test binary as the other tests do.

## Constraints

No new Go dependency and no new module. Do not change the behavior, output or
exit codes of existing commands, and do not edit dotfiles or write outside
temporary directories in tests. The helper must stay fast: a single directory
read, no network, no git. The generated scripts must not call `taskfactory`
for static data such as command and flag names. Keep usage text for existing
commands unchanged except for adding `completion` to the command list.

## Success criteria

### C1: Both scripts mention every command in the command table

Check: go test ./cmd/taskfactory -run 'TestCompletionScriptsListEveryCommand'

### C2: Adding a command or a flag without a completion entry fails a test

Check: go test ./cmd/taskfactory -run 'TestCompletionTableMatchesCommands'

### C3: The dynamic helper lists IDs from the right state directory

Check: go test ./cmd/taskfactory -run 'TestCompleteHelperIDsPerCommand'

### C4: The helper prints nothing and exits 0 outside a repository

Check: go test ./cmd/taskfactory -run 'TestCompleteHelperOutsideRepo'

### C5: The helper works from a subdirectory and filters by prefix

Check: go test ./cmd/taskfactory -run 'TestCompleteHelperSubdirAndPrefix'

### C6: An unknown shell or extra argument exits 2 and no shell exits 2

Check: go test ./cmd/taskfactory -run 'TestCompletionUsageErrors'

### C7: The bash script passes bash -n and drives real completions

Check: go test ./cmd/taskfactory -run 'TestBashCompletion'

### C8: The zsh script passes zsh -n, skipping with a message if zsh is absent

Check: go test ./cmd/taskfactory -run 'TestZshCompletionSyntax' -v

### C9: The tf alias is registered for completion in both scripts

Check: go test ./cmd/taskfactory -run 'TestCompletionRegistersTfAlias'

### C10: Help, hint table and top-level usage include completion

Check: go test ./cmd/taskfactory -run 'TestCompletionHelpAndHint'

### C11: bin/install prints the completion hint and edits no dotfile

Check: bin/install.test.sh default

### C12: The README documents the completion command

Check: grep -q 'taskfactory completion zsh' README.md

### C13: The full test suite passes

Check: bin/test

### C14: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run `go test ./cmd/taskfactory -run Complet -v` and report the zsh test
result, noting whether it ran or skipped. The bash test must source the real
script, set `COMP_WORDS` and `COMP_CWORD`, call `_taskfactory` and compare
`COMPREPLY` for these cases: `taskfactory pro` gives `promote`,
`taskfactory claim TF-` in a fixture lists ready IDs,
`taskfactory work TF-001 --adapter ` gives omp pi claude,
`taskfactory fail TF-001 --outcome ` gives FAILED BLOCKED,
`taskfactory requeue TF-001 --to ` gives ready inbox, `tf status --` offers
`--verbose`, `--dry-run` and `--help`. Then run `bin/install.test.sh`,
`bin/test`, `bin/lint` and `go run ./cmd/taskfactory validate tasks` from the
repository root and report each exit code. Also report the README grep result.

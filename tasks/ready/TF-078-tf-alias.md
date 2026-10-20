# TF-078: Install a short tf command next to taskfactory

## Goal

`bin/install` also installs a short global command `tf` that behaves exactly
like `taskfactory`, visible to scripts, non-interactive shells and the prompts
that `taskfactory work` builds, which a shell alias would not be.

## Dependencies

- TF-044

## Scope

Extend `bin/install` and `bin/install.test.sh`, the README and the skill.
Decisions are final:

1. `bin/install` installs `taskfactory` as today and, in the same destination
directory, a relative symlink `tf` pointing to `taskfactory`. Both always have
the same version, so `tf --version` equals `taskfactory --version`.
2. The alias is on by default. Setting `TASKFACTORY_NO_ALIAS=1` skips it.
`bin/install` still accepts no arguments and exits 2 otherwise.
3. Safety: `bin/install` never overwrites an existing `tf` unless it is already
a symlink to `taskfactory` in that directory, which it refreshes idempotently.
If a different file, directory or symlink named `tf` exists there, such as a
Terraform alias or binary, it installs `taskfactory`, skips the alias, prints a
clear warning on stderr naming the path, and exits 0. After installing, if
`command -v tf` resolves to a different file than the one installed, because an
earlier PATH entry shadows it, it prints a warning naming the shadowing path.
4. Program output stays unchanged. Usage, help and error text keep saying
`taskfactory` whatever name is invoked; the commands they suggest, such as
`taskfactory fail`, stay valid because `taskfactory` is installed too. No code
change in `cmd/taskfactory` is needed, and the test proves `tf --help` output
equals `taskfactory --help`.
5. The README Install section documents `tf`, the opt-out variable, the
do-not-overwrite rule and the shadowing warning. It says a shell alias
(`alias tf=taskfactory`) is only a fallback for people who cannot create the
symlink, because it is invisible to scripts and non-interactive shells. It says
plainly that `go install` of a remote path is not possible with the current
module name, that `bin/install` is the global install, and that a plain
`go install ./cmd/taskfactory` from a checkout installs only `taskfactory`
without the alias.
6. The skill and the README quick start mention that `tf` works anywhere
`taskfactory` does, with the full name still preferred in docs and generated
prompts.
7. Windows is out of scope because of symlinks; the README says so.
8. `bin/install.test.sh` runs one named scenario per argument and all of them
with no argument, which is what `bin/test` does. The scenarios are `default`,
`idempotent`, `optout`, `foreign`, `shadow`, `help` and `usage`. An unknown scenario
name exits 2 and each scenario prints a line starting with its name and PASS.

## Constraints

Touch only `bin/install`, `bin/install.test.sh`, `README.md` and
`skills/taskfactory/SKILL.md`. Do not change Go code, `bin/build` or the
program's usage text. Never overwrite a foreign `tf`, never modify the user's
shell profile or PATH, and keep the exit status 0 when the alias is skipped.
Tests use temporary directories only.

## Success criteria

### C1: Default install creates a relative tf symlink and versions match

Check: bin/install.test.sh default

### C2: A second install is idempotent

Check: bin/install.test.sh idempotent

### C3: The opt-out variable creates no symlink

Check: bin/install.test.sh optout

### C4: A foreign tf file or symlink is left untouched, with a warning

Check: bin/install.test.sh foreign

### C5: A shadowing tf earlier on PATH produces a warning

Check: bin/install.test.sh shadow

### C6: An extra argument to bin/install still exits 2

Check: bin/install.test.sh usage

### C7: The README documents the opt-out variable

Check: grep -q TASKFACTORY_NO_ALIAS README.md

### C8: tf --help output equals taskfactory --help

Check: bin/install.test.sh help

### C9: The full test suite passes

Check: bin/test

### C10: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run `bin/install.test.sh` with no argument and then with each scenario
name, `bin/test`, `bin/lint` and
`go run ./cmd/taskfactory validate tasks` from the repository root and report
each exit code. Also report the README grep result.

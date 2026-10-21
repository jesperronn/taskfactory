# TF-095: Embed the skill and add taskfactory skill install and status

## Goal

`taskfactory skill install` writes the agent skill that is embedded in the
binary into a project or user skills directory, and `taskfactory skill status`
reports whether that copy is present and current, with no network and no
release archive involved.

## Dependencies

- TF-084
- TF-085
- TF-094

## Scope

Add package `skills` in `skills/embed.go`, a package `internal/skillinstall`
with tests, dispatch and help in `cmd/taskfactory/main.go`, the hint table in
`nextstep.go`, the completion table, the README, the skill and
`docs/release.md`. Decisions are final:

1. Embedding. `skills/embed.go` is `package skills` and holds
`//go:embed taskfactory/SKILL.md` into an exported string. The canonical file
stays `skills/taskfactory/SKILL.md`, so there is no copy and no drift, the
`npx skills add` style layout is unchanged and `bin/skill.test.sh` keeps
passing. The Go file lives beside the skill directory, not inside it, so the
release skill archive, which packs `skills/taskfactory/`, is unchanged.
2. Commands: `taskfactory skill install` with flags `--target`
(`claude`, `agents` or `dir:<path>`), `--global` and `--force`, and
`taskfactory skill status` with `--target` and `--global`, plus the common
`--verbose` and `--dry-run` from TF-085.
`skill` with no or an unknown subcommand, an unknown flag or an extra
argument is a usage error on stderr in the form `taskfactory skill: ...` and
exits 2. `skill --help`, `skill install --help` and `skill status --help`
print help in the existing style, and `skill` is listed in the command table
and top-level usage.
3. Default target. Relative to the project root, found as other commands do,
the skill dir is `.claude/skills/taskfactory` when `.claude/` exists in the
project root and `.agents/skills/taskfactory` otherwise. `--target claude` and
`--target agents` force one, and `--target dir:<path>` uses `<path>` as the
skill directory itself. `--global` uses `$HOME/.claude/skills/taskfactory` or
`$HOME/.agents/skills/taskfactory` by the same rule applied to `$HOME`, and
combining `--global` with `dir:` is a usage error. Outside a project without
`--global` or `dir:`, install exits 1 with a clear message.
4. Install writes exactly `SKILL.md` and the stamp file `.taskfactory-skill`
into the skill dir. It refuses, exit 1, when the skill dir already exists and
names it, unless `--force`, which replaces that one directory and nothing
else. It writes into a temporary sibling and renames it so a failure leaves
no half-written skill. It never touches any other path, never runs git, never
commits and never reads or changes git signing or any config.
5. Stamp. The sidecar `.taskfactory-skill` has two lines,
`sha256 <hex of SKILL.md>` and `version <binary version>`. The frontmatter of
`SKILL.md` is never modified, so the installed file is byte-identical to the
embedded one.
6. Status. It resolves the skill dir like install and prints
`checked: <path>` for each path inspected, then one result line. Exit 0 when
`SKILL.md` exists and its SHA-256 equals the embedded one and the stamp
agrees, printed as `present`. Exit 1 when missing, printed as `missing`, or
when the hash differs or the stamp is absent or disagrees, printed as `stale`
with the installed and the embedded hash prefixes. Status never writes.
7. Dry run and verbose. `install --dry-run` prints `would:` lines naming each
directory and file and then `dry run: nothing changed`, and changes nothing.
`--verbose` prints each path as it is written.
8. Hints. Add `skill-install` and `skill-status` to the TF-084 hint table:
install success `taskfactory skill status`, install failure
`rerun with --force to replace it, or choose --target`, status failure
`taskfactory skill install`. `init` gains a fifth Next steps line,
`  5. Optional: install the agent skill with taskfactory skill install`,
followed by `(check with taskfactory skill status)` on the same line, built
from the hint table. `init` writes no skill and is otherwise unchanged.
9. TF-094 adds `completionSpecs`. This task adds a `skill` entry there: the
positional kinds `install` and `status`, flags `--target` with the fixed
values claude and agents, `--global`, and `--force` for install only.
10. Docs. README Agent skill section documents install, status, the targets,
`--global`, `--force`, and that the skill is embedded in the binary, so the
version of the binary decides the skill text. The skill gets a short note
that `taskfactory skill install` exists. `docs/release.md` says the skill
archive remains for `npx skills add` and manual use, not for the command.

## Constraints

TF-094 must land first, so that `completionSpecs` exists; that is why it is
listed under Dependencies. No new Go dependency and no network access. Tests
use temporary directories and set `HOME` to a temporary directory, never the
real home. Do not edit archived tasks, TF-079 or TF-080. Do not change
signing, the behaviour of other commands, or any output except the new fifth
init line, the usage list entry and the completion table. Keep list lines
under 80 columns.

## Success criteria

### C1: The embedded skill equals the canonical file

Check: go test ./skills -run TestEmbeddedSkillMatchesCanonicalFile -v

### C2: Default target picks .claude or .agents by project contents

Check: go test ./internal/skillinstall -run TestDefaultTarget -v

### C3: Targets, global and dir targets resolve as specified

Check: go test ./internal/skillinstall -run TestTargetsAndGlobal -v

### C4: Install writes the skill and stamp and nothing else

Check: go test ./internal/skillinstall -run TestInstallWritesOnlySkillDir -v

### C5: An existing directory is refused without force and replaced with it

Check: go test ./internal/skillinstall -run TestForce -v

### C6: Dry run prints would lines and changes nothing

Check: go test ./cmd/taskfactory -run TestSkillInstallDryRun -v

### C7: Status reports present, missing and stale with exit codes 0 and 1

Check: go test ./cmd/taskfactory -run TestSkillStatus -v

### C8: Usage errors exit 2

Check: go test ./cmd/taskfactory -run TestSkillUsageErrors -v

### C9: Help, hint table and top-level usage include skill

Check: go test ./cmd/taskfactory -run TestSkillHelpAndHints -v

### C10: init prints the fifth Next steps line and writes no skill

Check: go test ./cmd/taskfactory -run TestNextStepInitBlock -v

### C11: Completion table has a skill entry

Check: go test ./cmd/taskfactory -run TestCompletionTableMatchesCommands -v

### C12: The skill frontmatter check still passes

Check: bin/skill.test.sh

### C13: The README documents the commands

Check: grep -q 'taskfactory skill status' README.md

### C14: The full test suite passes

Check: bin/test

### C15: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run every check from the repository root, then `bin/lint` and report each exit
code. In a scratch project with `HOME` set to a temporary directory, run
`taskfactory skill status` (expect exit 1), `skill install --dry-run`,
`skill install`, `skill status` (expect 0), edit the installed `SKILL.md` and
rerun status (expect 1, stale), and paste the output.

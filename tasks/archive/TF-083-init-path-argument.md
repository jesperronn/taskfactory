# TF-083: Accept an optional path argument in taskfactory init

## Goal

`taskfactory init [path]` initializes the Git repository containing `path`, or
the one containing the current directory when `path` is omitted, so init can be
tried against another project from any folder.

## Dependencies

None

## Scope

Extend the init command in `cmd/taskfactory/main.go`, its tests, the README and
the skill. Decisions are final:

1. `path` names any directory inside a Git working tree, not only its root. The
target repository is resolved from it exactly as init resolves it from the
current directory today (`git -C <path> rev-parse --show-toplevel`, then
symlinks resolved). Without `path`, the current directory is used, so behavior
without arguments is unchanged.
2. `path` may be relative, which is resolved against the current directory, or
absolute. A symlink to a directory is followed, so a link to a repo root or a
subdirectory behaves like its target.
3. Exactly zero or one positional argument is accepted. Two or more exit 2 with
a usage error that names the extra argument. Flags `--help` and `-h` work
before or after `path` and print the init help with exit 0, writing nothing.
4. A `path` that does not exist, is not a directory, or is not inside a Git
working tree exits 1 with an error line on stderr that names the path as given
and says which of the three applies. Nothing is written in that case.
5. Every created or kept file lives in the target repository. The cwd
repository is never touched when `path` points elsewhere, even if the cwd is
itself a Git repository. Output stays empty on success.
6. `init --help` shows `Usage: taskfactory init [path]`, describes `path`, and
the top-level help command list shows `init [path]` with the same description
alignment as other entries.
7. The README (Init step and command list) and `skills/taskfactory/SKILL.md`
command table show `taskfactory init [path]` and say the path may be any
directory inside the project.
8. Tests are Go tests in `cmd/taskfactory/init_path_test.go` named
`TestInitPath...`. They build temporary Git repositories with
`commit.gpgsign=false` set in the repository's own config and never rely on
the user's global Git configuration.

## Constraints

Touch only `cmd/taskfactory/main.go`, `cmd/taskfactory/init_path_test.go`, the
existing tests that assert the old help text, `README.md` and
`skills/taskfactory/SKILL.md`. Do not change what init writes, other commands'
root discovery or exit codes, or commit signing. Do not edit
`docs/worker-instructions.md`.

## Success criteria

### C1: Without a path, init uses the current directory repository

Check: go test ./cmd/taskfactory -run TestInitPathDefaultCwd

### C2: An explicit repository root initializes that repository from elsewhere

Check: go test ./cmd/taskfactory -run TestInitPathRepoRoot

### C3: A subdirectory path initializes the containing repository root

Check: go test ./cmd/taskfactory -run TestInitPathSubdirectory

### C4: Relative and symlinked paths resolve to the target repository

Check: go test ./cmd/taskfactory -run TestInitPathRelativeAndSymlink

### C5: A nonexistent path or a file exits 1 and names the path

Check: go test ./cmd/taskfactory -run TestInitPathNotADirectory

### C6: A directory outside any Git repository exits 1 and names the path

Check: go test ./cmd/taskfactory -run TestInitPathNotInRepo

### C7: Two positional arguments exit 2

Check: go test ./cmd/taskfactory -run TestInitPathTwoPositionals

### C8: Help flags work before and after the path and the help shows init [path]

Check: go test ./cmd/taskfactory -run TestInitPathHelp

### C9: Initializing another path writes nothing in the cwd repository

Check: go test ./cmd/taskfactory -run TestInitPathNoWritesToCwdRepo

### C10: The README mentions init [path]

Check: grep -q 'init \[path\]' README.md

### C11: The skill mentions init [path]

Check: grep -q 'init \[path\]' skills/taskfactory/SKILL.md

### C12: The full test suite passes

Check: bin/test

### C13: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each named `go test` check, `bin/test`, `bin/lint` and
`go run ./cmd/taskfactory validate tasks` from the repository root and report
each exit code. Also report the README and skill grep result.

## Claim

Owner: sonnet-worker-083
Branch: task/TF-083
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-083
Base commit: 96f92a2b76614ff204ad86aa40f99e53eb7cffb0
Started at: 2026-10-20T22:39:55Z

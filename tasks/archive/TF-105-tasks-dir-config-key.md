# TF-105: Optional tasks_dir config key for the task folders

## Goal

A project can set `tasks_dir` in `.taskfactory/config.toml` to keep its task
folders somewhere else under the state root, for example `.taskfactory/tasks`,
while config, evidence, locks, logs and worktrees stay where they are. A
project without the key behaves exactly as before.

## Dependencies

None

## Scope

Decisions are final:

1. Key: a top-level string key `tasks_dir`, next to `protocol_version` and
`project_root` (not under a table). Default `"tasks"`. It is a slash path
relative to the state root, so it applies to in-project and external state
alike. Document it in `docs/config-v1.md` in the schema table and the complete
example, with an invalid example.
2. Validation, in one function in `internal/config` that both loaders call. An
empty string, an absolute path, a path with a `..` element, a path that is
`.`, a path with a backslash or a path that resolves to the state root, to
`.taskfactory` itself or to a directory that contains `.taskfactory` is
rejected. Errors name the key and the config file and
read `tasks_dir must be a relative path inside the state directory`. A
symlink in the path that leaves the state root is rejected too. Values are
cleaned (`./a//b/` becomes `a/b`). A tasks_dir equal to a runtime
subdirectory (`.taskfactory/evidence`, `logs`, `worktrees`,
`integration-evidence`) or inside one is rejected.
3. Bootstrap: the config file path stays fixed at
`<state>/.taskfactory/config.toml`. Add `config.TasksDirOf(stateRoot)`
beside `ProjectRootOf`, reading only `tasks_dir` with the same minimal TOML
decode and the same validation, returning the default when the file or key
is absent. Wherever a command discovers its roots and calls `paths.Register`
(and where `ProjectRootOf` is called today), it also resolves the tasks
directory and stores it in the `Layout` (new field `TasksDir`, empty meaning
the default). `paths.Default(root)` keeps the default.
4. `internal/paths`: `TasksRel`, `TaskDirRel` and `TaskFileRel` are package
functions today and cannot see a layout. Make them Layout methods (the
relative forms too) and change every caller (`cmd/taskfactory/main.go`,
`claim`, `fail`, `integrate`, `promote`, `requeue`, `taskvalidate`,
`workprompt`, completion) to take the layout from `paths.For(root)`. The
literals guard `TestNoPathLiterals` must stay green and must still skip only
`internal/paths`; no new `tasks` literal appears outside it.
5. Validation of the tree: `taskvalidate` lists states under the configured
directory. A directory under it that is not inbox, ready, active, failed or
archive, and any file placed directly in it other than `.gitkeep`, keeps
producing the existing `state` diagnostic, now showing the configured path,
for example `.taskfactory/tasks/other/x.md`. `validate` with a directory
argument equal to the configured directory behaves like `validate tasks` does
today (archive read for IDs only). A path argument outside it is the usual
usage error, naming the configured directory.
6. Dot directory: a tasks_dir under `.taskfactory` must be found, linted,
committed and not ignored everywhere. Check each place and fix it: `validate`
argument handling and listing, `claim` (dirs scan, active file move, the
worker checkout containing the contract), `promote`, `fail`, `requeue` and
`integrate` (they stage only task paths and the config, and the dirty-tree and
allowed-paths checks must allow the configured directory, not a literal),
`init`, completion's hidden `__complete` ids helper (it finds IDs and files
for the `taskfactory` argument completers), `workprompt` (the contract path in
the prompt and the active-file lookup), `bin/lint` (its `find` prunes
`.taskfactory`, `is_task_file` matches only `tasks/*.md`, and the `--tells`
scope lists `tasks/...`), `bin/test` (its whole-tree `validate tasks` step)
and `.mdsmith.yml`, whose `ignore` has `.taskfactory/**`. bin/lint.test.sh gets
a `tasksdir` scenario for this. bin/lint and bin/test
read the directory from the config (`taskfactory` has a way to print it; add
`taskfactory config tasks-dir` only if no existing command fits, or have the
scripts read the key with a small grep) and `.mdsmith.yml` ignore becomes
the explicit runtime subdirectories (`.taskfactory/worktrees/**`, `logs`,
`evidence`, `integration-evidence`) instead of `.taskfactory/**`, so Markdown
under `.taskfactory/tasks` is linted while worktree copies stay ignored.
7. `init` is unchanged by default and writes no `tasks_dir`. Add no required
flag. When the existing config already sets `tasks_dir`, `init` creates the
state folders at that place and keeps the key. The `.gitignore` lines init
writes stay the exact list in `paths.RuntimeIgnoreLines()`: they name specific
runtime subdirectories, so `.taskfactory/tasks` is not ignored. Add a test
that proves it with `git check-ignore`, and that an old project's
`.gitignore` is not rewritten.
8. End-to-end test in `cmd/taskfactory` or `internal/integrate`, using the
existing fixture helpers: a fixture git project whose config sets
`tasks_dir = ".taskfactory/tasks"`, committed in the default in-project mode.
It runs promote, claim, verify and integrate for one task and asserts: each
commit touches only paths under `.taskfactory/tasks` (plus the code change on
the task branch), no `tasks/` directory exists, `.taskfactory/evidence`,
locks, logs and worktrees are ignored (`git status --short` empty after
integrate), and `.taskfactory/config.toml` stays tracked and unchanged.
9. Docs: `docs/config-v1.md` (schema row, example, error text), a short
README note that task folders can be moved with `tasks_dir`, the `init`
and `--help` text that mentions `tasks/` where it names the folder
generically, `docs/worker-instructions.md` and `docs/task-state-options.md`
(one sentence each pointing at the key). Archived tasks stay untouched.

## Constraints

No behaviour change for any config without `tasks_dir`: existing tests pass
unmodified except where a test calls a function whose signature moved, and the
default layout produces byte-identical paths, messages and commits. Do not
change signing, commit identity, `project_root` semantics or the config file
location. Do not move this repository's own tasks; TF-104 does that. Keep
`TestNoPathLiterals` green without widening its skip list. Never add a new
dependency.

## Success criteria

### C1: tasks_dir validation accepts and rejects the right values

Check: go test ./internal/... -run TasksDirConfig -count=1

### C2: TasksDirOf reads only the key and defaults when absent

Check: go test ./internal/... -run TasksDirOf -count=1

### C3: Layout paths follow tasks_dir and default to tasks

Check: go test ./internal/paths -count=1

### C4: Production code still spells no task path literal

Check: go test ./internal/paths -run TestNoPathLiterals -count=1

### C5: validate finds files and rejects unknown dirs under .taskfactory/tasks

Check: go test ./internal/... -run TasksDirValidate -count=1

### C6: promote, fail and requeue move and commit under the configured dir

Check: go test ./internal/... -run TasksDirLifecycle -count=1

### C7: claim and workprompt use the configured directory

Check: go test ./internal/... -run TasksDirClaim -count=1

### C8: integrate archives under the configured directory

Check: go test ./internal/... -run TasksDirIntegrate -count=1

### C9: init keeps the key and .taskfactory/tasks is not git-ignored

Check: go test ./cmd/... -run 'TasksDirInit|TasksDirIgnore' -count=1

### C10: The completion ids helper lists tasks under the configured dir

Check: go test ./cmd/taskfactory -run 'TasksDirComplete' -count=1

### C11: The end-to-end loop works with tasks_dir under .taskfactory

Check: go test ./cmd/taskfactory -run 'TasksDirEndToEnd' -count=1

### C12: bin/lint lints Markdown under a dot-directory tasks dir

Check: bin/lint.test.sh tasksdir

### C13: docs/config-v1.md documents the key

Check: grep -q tasks_dir docs/config-v1.md

### C14: The full test suite passes

Check: bin/test

### C15: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each check, `bin/test` and `bin/lint` from the repository root and report
each exit code. Also report `go test ./... -count=1` and the paste of the
end-to-end test's `git log --stat` assertions passing.

## Claim

Owner: sonnet-worker-105
Branch: task/TF-105
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-105
Base commit: 295a79a6377721e54f23112dfeefc52bcf6c25cb
Started at: 2026-10-21T15:42:23Z

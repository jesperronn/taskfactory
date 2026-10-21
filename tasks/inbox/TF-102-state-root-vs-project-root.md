# TF-102: Separate stateRoot from projectRoot

## Goal

TaskFactory can keep task state, evidence, locks and config in a directory
outside the code repository, while a project without the new setting behaves
exactly as today.

## Dependencies

None

## Scope

Product behaviour only; this repository's own data does not move here.

1. Two roots. `projectRoot` is the code repository: branches, worktrees,
`merge --ff-only` and the checks run there. `stateRoot` holds `tasks/`,
`.taskfactory/` evidence, locks, logs and `config.toml`.
2. Discovery order for `stateRoot`: the `--state-dir <path>` flag, then the
`TF_STATE_DIR` environment variable, then the default, which is the project
root. Config key `project_root` in the state config names the code repository
when `stateRoot` differs; it is required then and an error names it if absent.
3. A single path-helper package, for example `internal/paths`, replaces every
`tasks/{inbox,ready,active,failed,archive}` and `.taskfactory/...` literal in
promote, fail, requeue, integrate, verify, workprompt, taskvalidate, claim,
`cmd/taskfactory/main.go` and completion. A test fails if a literal returns.
4. Git operations in claim, verify and integrate use `projectRoot`. Commit
behaviour of moves stays as today for the default layout; the state-repo
commit policy is TF-103.
5. The default layout is unchanged: same paths, messages, exit codes and
output. `--state-dir` joins the completion table; help, next-step guidance,
`--dry-run` and `--verbose` conventions apply to it.

## Constraints

Order: land after TF-101 and before TF-103 and TF-104 (not a validator
dependency, since dependencies must be archived). Do not edit `tasks/` content.
Never touch git signing. Keep list lines short and add no new top-level
dependencies.

## Success criteria

### C1: The default layout is unchanged

Check: go test ./internal/paths -run TestDefaultLayout -count=1

### C2: Discovery prefers flag over env over default

Check: go test ./cmd/taskfactory -run TestStateDirDiscovery -count=1

### C3: Claim, verify and integrate work with an external plain state dir

Check: go test ./internal/integrate -run TestExternalStateDir -count=1

### C4: The project tree stays clean during that run

Check: go test ./internal/integrate -run TestExternalStateCleanProject -count=1

### C5: No hardcoded task path literal remains

Check: go test ./internal/paths -run TestNoPathLiterals -count=1

### C6: The completion table covers the flag

Check: go test ./cmd/taskfactory -run TestCompletionStateDir -count=1

### C7: The full test suite passes

Check: bin/test

### C8: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each check, `bin/test` and `bin/lint` from the repository root and report
each exit code.

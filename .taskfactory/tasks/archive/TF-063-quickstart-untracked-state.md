# TF-063: Quick start leaves main dirty with untracked .taskfactory files

## Goal

After `integrate`, a project that followed the README has an empty
`git status --short`.

## Dependencies

None

## Scope

`taskfactory init` writes the project `.gitignore`, or appends to an existing
one, with exactly these runtime paths under one comment line:
`.taskfactory/claim.lock`, `.taskfactory/integration.lock`,
`.taskfactory/evidence/`, `.taskfactory/integration-evidence/`,
`.taskfactory/logs/` and `.taskfactory/worktrees/`. It never ignores
`.taskfactory/config.toml`, never duplicates a line on a rerun and never
touches other lines. The README quick start says to commit the config and the
`.gitignore` together before the first claim. Claim, work, verify, integrate
and check-main must keep working with these paths ignored.

## Constraints

No signing override in product code. Do not weaken the rule that integrate
needs a tracked, unchanged config. Change `internal/claim` or
`internal/integrate` only if an experiment shows they break on ignored paths.

## Success criteria

### C1: Init writes and keeps the ignore lines

Check: go test ./cmd/taskfactory -run TestInitGitignore -v

### C2: Integrate works with the paths ignored

Check: go test ./internal/integrate -run TestIntegrateWithIgnoredRuntime -v

### C3: The README says to commit config and gitignore

Check: go test ./cmd/taskfactory -run TestReadmeQuickStartGitignore -v

## Verification

Run each check from the repository root with -v and report that it shows PASS.
Run `bin/test` and `bin/lint` and report their exit codes. Paste the
`.gitignore` after two `init` runs and the empty `git status --short` from an
end-to-end run in a scratch repository.

# TF-055: Worker prompt builder and run log layout

## Goal

Provide the two pure building blocks a `work` command needs: a function that
turns a claimed task into the single prompt string an adapter receives, and a
function that names and opens the per-run log file under `.taskfactory/`. No
process is launched and no lifecycle state changes.

## Dependencies

- TF-009
- TF-011
- TF-024
- TF-025
- TF-026

## Scope

Add a new package `internal/workprompt`. It touches no other package and no
file under `cmd/`.

1. `Build(projectRoot, id string) (Prompt, error)`. Find exactly one file for
   `<id>` under `tasks/active/`. Require a complete `## Claim` block and
   return `Prompt{Text, Worktree, Branch, BaseCommit}` taken from it. `Text`
   is, in order: a short header naming the task ID, branch and worktree; the
   task file bytes verbatim; the bytes of `docs/worker-instructions.md` read
   from the claimed worktree; and a closing rules block. The closing block
   says: work only inside the worktree; commit the result with plain
   `git commit` and never change signing; do not run `integrate`, `fail` or
   `promote`; run the task checks, `bin/test` and `bin/lint`, and report each
   exit code; stop and report a blocker instead of weakening a check.
2. `Build` is deterministic: the same inputs give the same bytes. It reads
   files only and never runs a command.
3. `LogPath(projectRoot, id, adapter string, at time.Time) string` returns
   `<projectRoot>/.taskfactory/logs/<id>/<adapter>-<UTC>.log` where `<UTC>` is
   `20060102T150405Z`. `OpenLog(path, header)` creates the parent directories
   with mode 0755, creates the file with `O_EXCL` and mode 0644, writes a
   header line block (task ID, adapter, model, timeout, start time) and
   returns the file. It refuses to overwrite an existing log.
4. Refuse with a clear error: a task not in `tasks/active/`, an incomplete
   Claim block, a missing `docs/worker-instructions.md` in the worktree, and an
   empty or unsafe adapter name (only `omp`, `pi`, `claude` are accepted).

## Constraints

Standard library and existing internal packages only. Never write the model
endpoint credentials, `ANTHROPIC_AUTH_TOKEN` or any environment value into the
prompt or the log header. Never override Git signing. Do not edit the task
file, evidence files or any file under `tasks/`. The prompt must not tell the
worker to integrate, archive or push. Logs live under `.taskfactory/` and are
never staged.

## Success criteria

### C1: Build returns the claim fields and the verbatim task text

Check: go test ./internal/workprompt -run Build

### C2: Build is deterministic and reads files only

Check: go test ./internal/workprompt -run Deterministic

### C3: Build refuses a missing, unclaimed or incomplete task

Check: go test ./internal/workprompt -run Refuses

### C4: The closing rules forbid integrate, fail, push and signing changes

Check: go test ./internal/workprompt -run Rules

### C5: LogPath and OpenLog use the documented layout and never overwrite

Check: go test ./internal/workprompt -run Log

### C6: No secret value reaches the prompt or log header

Check: go test ./internal/workprompt -run NoSecrets

### C7: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each C-check, then `bin/test` and `bin/lint` from the repository root and
report each exit code. Paste the prompt produced for a fixture task up to the
end of the header, and the log path produced for a fixed timestamp.

**Notes (not part of the contract):**

Open questions for the owner (the defaults shown are the planner's proposal):

- Where do prompts and logs live? Proposal: only the log is stored, at
  `.taskfactory/logs/<ID>/<adapter>-<UTC>.log`. The prompt is rebuilt from the
  task file on demand and not stored. Alternative: also store
  `<adapter>-<UTC>.prompt.md` next to the log so a run can be replayed exactly.
- Should the worker instructions be read from the worktree (the base commit
  version) or from the project root? Proposal: the worktree, so the worker sees
  what it will be checked against.
- Should the log header record the model id? Proposal: yes. It is not a secret.
- Is `.taskfactory/logs/` ignored by Git? `.gitignore` does not hide
  `.taskfactory/` today, so logs show as untracked, like evidence.

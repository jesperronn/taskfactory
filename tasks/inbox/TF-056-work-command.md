# TF-056: Work command that launches a local worker on a claimed task

## Goal

Add `taskfactory work <ID> --adapter <omp|pi|claude> --model <id>
[--timeout <duration>]` so TaskFactory itself launches the chosen adapter in a
claimed task's worktree. The command never commits, verifies, integrates or
fails a task for the worker.

## Dependencies

- TF-024
- TF-025
- TF-026
- TF-048
- TF-055

## Scope

New package `internal/work`, a thin dispatch in `cmd/taskfactory/main.go`, a
`workHelp` constant and entries in `commandSummaries` and `commandHelps`.

Behavior:

1. Usage: `--adapter` and `--model` are required and explicit. There is no
   default adapter, no default model and no fallback. `--timeout` defaults to
   10m and must be at least one second. Exactly one adapter is accepted; a
   repeated or second `--adapter` is exit 2. Unknown arguments are exit 2.
2. Require exactly one active task with a complete Claim block and a
   registered worktree (see `workprompt.Build`, TF-055). Refuse with exit 1
   otherwise, before any process starts.
3. Run the adapter preflight for the chosen adapter. A refused preflight prints
   each failed check, makes no change to the worktree, tasks or evidence, and
   exits 1. The command never picks another model or adapter.
4. Build the prompt with `workprompt.Build`, open the log with
   `workprompt.OpenLog`, and call the adapter `Run` with the worktree from the
   Claim and the explicit model. Claude also needs `--haiku-model`; see Notes.
5. Progress: copy the adapter output to the log when the run ends and print the
   log path at the start. Live streaming is not required in this task (the
   adapters capture output; see Notes).
6. Map the `common.Result` to exit codes and a printed next step:
   - `exit` with code 0: print `taskfactory verify <ID>` as the next step,
     exit 0. A zero exit is a claim by the worker, not a pass.
   - `exit` with a non-zero code: print the log path, exit 1.
   - `blocked` or `stalled`: print the note and the log path, then print the
     command `taskfactory fail <ID> --outcome BLOCKED --reason "<note>"` as a
     suggestion only, exit 1. It does not run it.
7. Print the commit the worker left in the worktree with `git log -1`
   information only if the head differs from the Claim base commit. Never
   create or amend a commit.

Exit codes: 0 worker exited 0; 1 refused, blocked, stalled or worker failed;
2 invalid usage. `work --help` and `work -h` print `workHelp` and exit 0.

## Constraints

Do not change `claim`, `verify`, `fail`, `integrate`, `promote` or any adapter
package behavior. Do not run `fail`, `verify` or `integrate` from `work`. Do
not commit, stage or edit any file in the project root or the worktree; the
only file `work` writes is its log. Never override Git signing. Claude runs
with the accept-edits permission mode the adapter already sets; `work` does not
change permissions. Never print or log credentials. No new Go dependencies.
Tests inject a fake adapter runner so no model, network or binary is used.

## Success criteria

### C1: Missing or repeated flags and unknown arguments exit 2

Check: go test ./cmd/taskfactory -run Work

### C2: A refused preflight exits 1 and changes nothing

Check: go test ./internal/work -run Preflight

### C3: A task that is not active and claimed is refused before launch

Check: go test ./internal/work -run Refuses

### C4: A zero exit prints the verify step and runs nothing else

Check: go test ./internal/work -run ExitZero

### C5: Blocked and stalled print the fail suggestion and do not run it

Check: go test ./internal/work -run Suggests

### C6: The run log is written under .taskfactory/logs and nothing is staged

Check: go test ./internal/work -run Log

### C7: No non-test file in internal/work overrides signing or commits

Check: go test ./internal/work -run CheckNoCommit

### C8: The whole task tree validates

Check: go run ./cmd/taskfactory validate tasks

## Verification

Run each C-check, then `bin/test` and `bin/lint` and report each exit code. In
a scratch repository with a fake adapter binary on PATH, claim a task and run
`work` for an exit 0, an exit 1 and a timeout. Report each exit code, the
printed next step and `git status` before and after.

## Notes

Open questions for the owner:

1. Does `work` ever run `fail` itself on a stall? Proposal: no. It only prints
   the suggested command; the owner (or orchestrator) decides, because a stall
   may be worth a longer timeout. An opt-in `--fail-on-stall` flag could come
   later.
2. How is progress shown? The adapters return output only after the run ends.
   Proposal: v1 prints the log path and writes output at the end; a later task
   adds streaming (`stream-json` for Claude, rpc modes for omp and pi) if the
   owner wants live output.
3. Model id validation: the adapters already check the id against the harness
   catalog or the oMLX `/v1/models` list. Proposal: `work` adds no extra check
   beyond non-empty and no whitespace.
4. Two adapters requested: proposal is exit 2. Running two at once also slows
   the single oMLX server (see `docs/experiments/local-harness-trials.md`).
5. Claude needs a second explicit id for the haiku tier. Proposal: a required
   `--haiku-model` flag when `--adapter claude`, exit 2 if it is missing.
6. Endpoint: proposal is the adapter default `127.0.0.1:8000` and a
   `--endpoint host:port` flag. Should it come from `.taskfactory/config.toml`
   instead? That needs a config change and is out of scope here.
7. After a worker exit 0, is a worker commit required before `verify`? The
   worker is told to commit; `work` only reports whether the head moved.
8. Should a failed run (non-zero exit) also suggest `fail --outcome FAILED`?
   That needs a FAILED evidence record, which only `verify` writes, so
   proposal: suggest `taskfactory verify <ID>` first.

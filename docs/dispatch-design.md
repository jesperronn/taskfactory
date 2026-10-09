# Dispatch design: launching a worker from TaskFactory

The three adapters (`omp`, `pi`, `claude`) exist as Go packages, but no CLI
command launches one. The loop claim, work, verify, integrate cannot be driven
by TaskFactory alone. Worker selection stays manual in v1: the operator names
the adapter and the model, with no default and no fallback.

## Planned tasks

- TF-055: prompt builder and log layout (`internal/workprompt`).
- TF-056: `taskfactory work <ID> --adapter A --model M [--timeout D]`.
- TF-057: README quick start and one recorded end-to-end run.
- TF-058: opt-in smoke test against the local oMLX server, skipped if down.

Order: TF-055, then TF-056, then TF-057 and TF-058.

## State machine, claim to archive

| Step | State change             | Command                    | Who    |
| ---- | ------------------------ | -------------------------- | ------ |
| 1    | inbox to ready           | `promote <ID>`             | human  |
| 2    | ready to active          | `claim <ID> --owner N`     | auto   |
| 3    | worker runs              | `work <ID> --adapter ...`  | human  |
| 4    | worker commits           | worker, plain `git commit` | worker |
| 5    | independent check        | `verify <ID>`              | auto   |
| 6    | active to failed         | `fail <ID> --outcome O`    | human  |
| 7    | failed to ready or inbox | `requeue <ID>`             | human  |
| 8    | active to archive        | `integrate <ID>`           | human  |
| 9    | main recheck             | `check-main`               | auto   |

Steps 5 and 9 are mechanical once started. Every state change that loses or
retries work is a human decision.

## What `work` does

1. Requires one active claimed task and explicit adapter and model.
2. Runs adapter preflight: binary, model in catalog, endpoint reachable. A
   refusal exits 1 before launch and changes nothing.
3. Builds the prompt from the task file, the worker instructions (the project's
   `docs/worker-instructions.md`, or a built-in text when it is missing) and a
   short rules block (TF-055), and opens a log under `.taskfactory/logs/<ID>/`.
4. Launches the adapter in the claimed worktree and waits.
5. Maps the adapter result:
   - exit 0: prints `taskfactory verify <ID>`. A zero exit is a claim only.
   - non-zero exit: prints the log path, exit 1.
   - blocked or stalled: prints the suggested
     `fail <ID> --outcome BLOCKED --reason ...` and does not run it.

`work` never commits, verifies, integrates or fails a task. It writes only its
log. A BLOCKED evidence record must exist before `fail --outcome BLOCKED`
accepts, or a `--reason` is given (TF-048).

## Fixed rules

- TaskFactory runs plain `git commit` and never overrides commit signing.
- Archived tasks are history and are not edited.
- The Claude adapter keeps its accept-edits permission mode.
- Worker output is a claim; only `verify` evidence counts.
- Run one local worker at a time; they share one model server.

## Open decisions for the owner

1. Does `work` ever run `fail` itself on a stall? Planned: no.
2. How is progress shown? Planned: log path, output written at the end.
3. Where do prompts and logs live? Planned: logs only, under `.taskfactory/`.
4. How are model ids validated? Planned: by adapter preflight only.
5. Two adapters requested? Planned: usage error, exit 2.
6. Claude needs a second id for its haiku tier: a `--haiku-model` flag?
7. Endpoint: a `--endpoint` flag or the config file?

Details and proposals are in the Notes of TF-055 and TF-056.

# TF-110: Per-task retry budget for requeue

## Goal

Stop a task from being requeued forever. A configurable budget limits how many
attempts a task may use. `taskfactory requeue` refuses a task that has used up
its budget and says why.

## Dependencies

- TF-049

## Scope

Add a budget key to the config model in `internal/config/config.go` with a
safe default. Enforce it in `internal/requeue/requeue.go` inside
`RequeueWith`, using the last attempt number from `planEvidence`. Document the
key in `docs/config-v1.md` and the retry wording in `README.md` near the
"allowed execution/retry budget" text. The budget is checked only by requeue.
Workers and `work` do not read it in this task.

## Constraints

A refusal changes no file and creates no commit. The refusal message names
the task ID, the attempts used and the budget. An unset key keeps today's
behaviour unless the default is documented as a limit. Use only the Go
standard library.

## Success criteria

### C1: A named test refuses a requeue past the budget

Check: go test ./internal/requeue -run TestRequeueRefusesPastRetryBudget

### C2: A named test parses the budget key

Check: go test ./internal/config -run TestRetryBudgetKeyParses

### C3: The whole test suite passes

Check: bin/test

### C4: Lint passes on the repository

Check: bin/lint

## Verification

Run each check from the repository root and report each exit code. Report
the default budget value chosen and where it is documented.

## Claim

Owner: haiku-worker-110
Branch: task/TF-110
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-110
Base commit: 1e41b58426e21bf441673fafd0635d67f5baf7a9
Started at: 2026-10-22T12:22:42Z

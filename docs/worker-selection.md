# Worker selection exploration

This document asks which recorded evidence would justify automatic worker
selection, and whether TaskFactory should build it. It is a document only. It
makes no code change. Worker selection stays manual in v1, as
`docs/local-workers-v1.md` requires: the operator names the adapter and the
model, and the adapter never falls back.

## Question

Should TaskFactory choose a worker (adapter and model) for a ready task
automatically, and if so, what recorded evidence would have to exist first?

Open questions, all open:

- Open: does the choice of adapter change the outcome of a task, at equal task
  difficulty? The recorded data cannot answer this yet (see below).
- Open: does the choice of model change the outcome or the duration? The only
  model comparison on record is small and ran on a different task family.
- Open: what is a fair difficulty measure for a task, so that outcomes can be
  compared across tasks?

## Recorded data

Sources read in this repository:

- Worker verification evidence, `.taskfactory/evidence/<ID>.jsonl`. Schema in
  `docs/task-format-v1.md`. Untracked, not in git.
- Integration evidence, `.taskfactory/integration-evidence/<ID>.jsonl`.
- Experiment records in `docs/experiments/`.
- Work logs, `.taskfactory/logs/`. This directory does not exist in the checkout,
  so there is no work log data to use.

### What the evidence schema contains

Each attempt line has these keys: `task_id`, `attempt`, `recorded_at`,
`outcome`, `branch`, `base_commit`, `result_commit`, `changed_files`, `checks`
and `note`. The schema has no `adapter`, no `model` and no duration field.
Each `checks` entry records the exit code of one check, not how long it ran.

This means the fields the task asks us to judge can be read as follows:

| Field    | In worker evidence? | Notes                                                                   |
| -------- | ------------------- | ----------------------------------------------------------------------- |
| outcome  | Yes                 | `PASS`, `FAILED` or `BLOCKED`                                           |
| attempts | Yes                 | Counted from the `attempt` number for each task                         |
| adapter  | No                  | Not recorded in any evidence line                                       |
| model    | No                  | Not recorded in any evidence line                                       |
| duration | No                  | Only `recorded_at` per attempt. Gaps between attempts are not run times |

### Evidence counts

Counted from the 26 evidence files present in the checkout, on the main
checkout's `.taskfactory/evidence/`:

- 26 tasks, 61 attempt records.
- Outcomes: 52 `PASS`, 9 `FAILED`, 0 `BLOCKED`.
- 19 of 26 tasks passed on their first attempt. One task (TF-104) has a single
  attempt, which passed.
- 7 tasks needed a retry: TF-081 (1 failure), TF-088, TF-089, TF-090, TF-091,
  TF-108 (1 failure each), and TF-092 (3 failures before a pass).
- All 26 tasks end on a `PASS` attempt.
- No `requeue` archive files (`<ID>.attempts-<N>.jsonl`) exist, so no attempt
  counter has been reset.
- Integration evidence records a different shape: `stage`, `main_before`,
  `verified_commit` and similar keys. It also has no adapter, model or duration
  field.

Because the worker evidence has no adapter or model field, these counts cannot
be split by worker. Every attempt in the checkout is treated as one unknown
worker.

### Adapter and model data in experiments

The experiment records are the only place where adapter and model appear. They
are hand-run trials, not evidence lines:

- `docs/experiments/TF-039-readability.md`, 35B model
  (`Ornith-1.5-35B-A3B-MLX-4bit`), adapter `pi`, `--timeout 8m`: 20 runs on
  types 4 and 5, 20 of 20 passed in both conditions. Median wall time was 113 s
  in condition A and 124.5 s in condition B, with a spread of 71 to 272 s. The
  record concludes the comparison is inconclusive.
- The same record, 9B model (`Ornith-1.5-9B-MLX-4bit`): 4 runs, 0 valid passes.
  Each ran into the 8 minute timeout (480 to 481 s), with the work exit at 1 and
  no commit. No cell has the five runs the design asks for.
- `docs/experiments/local-harness-trials.md`: a single trial with each adapter
  on the same small task. `pi` 7 s, `omp` 19 s, `claude` 12 s. On a longer
  lint task, `pi` finished in 22 s, `omp` reached the 600 s limit and `claude`
  is not timed. These are one run each.
- `docs/experiments/TF-015-worker-comparison.md`: three adapters on one task,
  one run each. Two runs stopped without a result, and one candidate failed
  verification.
- `docs/experiments/e2e-local-worker.md` and `quickstart-verbatim-rerun*.md`
  show the `pi` adapter with the 35B model, one run per record.

### What the recorded data cannot support

- No claim that one adapter is better than another. Worker evidence has no
  adapter field, and the experiment runs are too few and differ in task.
- No claim that a 35B model is better than a 9B model in general. The 9B result
  is 0 of 4 valid passes, all on timeouts, and the 35B result is at ceiling on
  its only comparison.
- No duration claim from evidence files. `recorded_at` is the time the attempt
  was recorded, not the run's duration, and gaps between attempts include
  human time.
- No claim about difficulty. The 26 tasks are not labelled by difficulty, so the
  pass rate cannot be compared across them.

## Evidence threshold

Automatic selection would need recorded evidence that does not exist yet. The
threshold below is a proposal. It has not been measured, and it is not in any
TaskFactory code or config.

Before a selection rule is considered, the evidence should include:

1. An `adapter` and a `model` field in each attempt record, and a run duration
   field. This is a schema change, which would need its own task and a protocol
   version decision. Until it exists, no attempt can be attributed to a worker.
2. At least five attempts per (adapter, model) cell on a shared set of tasks of
   the same difficulty, which is the design `TF-039-readability.md` already
   names as its next step.
3. A difference in pass rate or median duration between cells that exceeds the
   spread seen in the runs. The TF-039 record shows a spread of 71 to 272 s on
   a single cell, so a 11 s median gap is not enough.
4. `BLOCKED` outcomes recorded as their own result. None are in the checkout
   today, so the rate of preflight refusals is unknown.

Open: the number of tasks and the task-difficulty measure needed to meet 2 and 3
are not known.

## Recommendation

Do not build automatic worker selection now. The recorded data has no adapter,
model or duration field, and the only model comparison is inconclusive, so the
evidence cannot justify it. Keep selection manual in v1, and if the question is
reopened, first add `adapter`, `model` and duration to the attempt schema and
collect at least five runs per cell on a shared task set.

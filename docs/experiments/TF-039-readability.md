# TF-039: Readability for worker-facing text

Question: does shorter, simpler worker-facing text measurably help a small
local model finish tasks? This branch measures the text, simplifies two
files, and prepares the experiment. **The worker experiment was not run**:
the local model server was down (see "Experiment: not measured"). There is
no evidence either way that simpler text helps the worker.

## Method

The advisory command is `bin/lint --readability`. It reads
`.mdsmith-readability.yml`, runs only MDS023 (paragraph-readability), MDS024
(paragraph-structure) and MDS036 (max-section-length) with the pinned
mdsmith v0.57.0, prints the findings and a `stats:` line, and always exits 0.
The default `bin/lint` and `.mdsmith.yml` are unchanged, so
`paragraph-readability: false` still holds. Findings stay out of the lint
exit code because most measured files do not meet the thresholds yet.

Scope: `tasks/inbox`, `tasks/ready`, `tasks/active`, `tasks/failed`,
`docs/worker-instructions.md`, `docs/protocol-v1.md`,
`docs/task-format-v1.md`, `skills/taskfactory/SKILL.md` and `README.md`
(its quick start is lines 95 to 257). `tasks/active` and `tasks/failed` are
empty today. Nothing under `tasks/archive` is read.

Rule settings, from `mdsmith help rule`:

- MDS023 uses the Automated Readability Index (ARI): `max-index` (default
  14.0, about US college level) and `min-words` (default 20; shorter
  paragraphs are skipped).
- MDS024 uses `max-sentences` (default 6) and `max-words-per-sentence`
  (default 40).
- MDS036 uses `max` lines per section (default 0, off), `per-level`,
  `max-words` and `max-paragraphs`.

Tables and code blocks are skipped by MDS023 and MDS024. MDS036 counts all
lines of a section, including code blocks.

## Thresholds

| File kind            | MDS023 max ARI | MDS024 sent. | MDS024 words | MDS036 |
| -------------------- | -------------- | ------------ | ------------ | ------ |
| Worker instructions  | 12.0           | 5            | 30           | 40     |
| Task files and specs | 14.0           | 6            | 40           | 60     |

Worker instructions are `docs/worker-instructions.md` and
`skills/taskfactory/SKILL.md`: a small model reads them before every task,
so they get the strict values. Task files, the protocol and task-format
specs and the README keep the mdsmith defaults for MDS023 and MDS024, which
the corpus mostly meets (mean ARI 11 to 13). MDS036 is set to 60 lines for
those kinds because 40 flagged ordinary sections that are mostly examples.
The worker-instruction values were chosen because the rewritten file could
meet them without losing content (maximum ARI 11.8, below 12.0).

Caveat: ARI counts characters per word, so paragraphs full of paths and
commands score high even when the sentences are short. The two strict
paragraphs in `SKILL.md` stayed under 12 only after the sentences were made
very short.

## Findings per file kind

MDS023 index values are over paragraphs of 20 words or more (count, mean,
maximum). The first block is the original text, the second is this branch.

| File                      | Paras | Mean ARI | Max ARI |
| ------------------------- | ----- | -------- | ------- |
| README.md                 | 23    | 11.0     | 21.8    |
| docs/protocol-v1.md       | 23    | 13.2     | 17.9    |
| docs/task-format-v1.md    | 17    | 10.9     | 16.1    |
| worker-instructions, orig | 6     | 11.2     | 13.2    |
| worker-instructions, new  | 14    | 8.6      | 11.8    |
| SKILL.md, orig            | 3     | 13.4     | 18.0    |
| SKILL.md, new             | 3     | 5.5      | 8.4     |
| tasks/inbox (2 files)     | 3     | 13.7     | 17.2    |
| tasks/ready (TF-039)      | 5     | 15.8     | 23.7    |

Finding counts, before (original files) and after (this branch), as
MDS023 / MDS024 / MDS036:

| File kind                   | Before    | After     |
| --------------------------- | --------- | --------- |
| tasks/ready (1 file)        | 4 / 1 / 0 | 4 / 1 / 0 |
| tasks/inbox (2 files)       | 2 / 0 / 0 | 2 / 0 / 0 |
| tasks/active, tasks/failed  | 0 / 0 / 0 | 0 / 0 / 0 |
| docs/worker-instructions.md | 3 / 5 / 0 | 0 / 0 / 0 |
| skills/taskfactory/SKILL.md | 2 / 0 / 1 | 0 / 0 / 1 |
| docs/task-format-v1.md      | 3 / 9 / 2 | 3 / 9 / 2 |
| docs/protocol-v1.md         | 9 / 9 / 1 | 9 / 9 / 1 |
| README.md (all)             | 5 / 7 / 2 | 5 / 7 / 2 |
| README.md quick start only  | 2 / 6 / 1 | 2 / 6 / 1 |
| Total                       | 28/31/6   | 23/26/6   |

The `stats:` lines were `MDS023=28 MDS024=31 MDS036=6` before and
`MDS023=23 MDS024=26 MDS036=6` after. The README (all) row
includes its quick start. The one remaining `SKILL.md` finding is a 50 line
section ("Ready task contract") that is mostly the task template.

## Simplification

Files changed (prose only; no IDs, commands, `Check:` lines or exit codes):

- `docs/worker-instructions.md`: shorter sentences, paragraphs split to at
  most five sentences, evidence paths moved into a list. The original is in
  Git at `main:docs/worker-instructions.md`.
- `skills/taskfactory/SKILL.md`: two paragraphs (the intro and the start of
  "Ready task contract") reworded. `bin/skill.test.sh` still passes.

Not changed: protocol, task-format, README and the task files. They are
large, normative or recorded evidence, and rewording them was not needed to
answer the question. The archive is untouched.

## Experiment: not measured

Plan: three tiny task types (fix a return value so a test passes, add a
function a test already calls, fix gofmt and vet complaints), two
conditions (A original instructions and verbose task, B simplified
instructions and short task, same requirements), three runs each, 18 runs,
one at a time on pi with `Ornith-1.5-35B-A3B-MLX-4bit`.

Blocker: before starting, `curl -s -m 3 http://127.0.0.1:8000/v1/models`
returned nothing (HTTP code 000), `lsof` showed no listener on port 8000 and
no omlx or mlx process was running. A `taskfactory work` preflight confirmed
`endpoint 127.0.0.1:8000 is unreachable ... connection refused`. The
worker did not start the server. Zero worker runs were made, so there is no
comparison table and the answer to the question is unknown.

What is ready to run (not run against a model):

- `docs/experiments/TF-039-run.sh BINARY A|B 1|2|3` builds a scratch repo
  with local Git config, copies the original (A, from `main`) or simplified
  (B) instructions, writes the task, then runs promote, claim, work and
  verify and prints one result line.
- `docs/experiments/TF-039-fixtures/t1-A.task` to `t3-B.task` are the six
  task texts. B has the same goals, scope, constraints and `Check:` lines as
  A, in short sentences.
- Setup check: with the server down, each of the six cases reached the work
  step, preflight refused, and `verify` then reported the expected FAIL for
  an unfixed task (1 check failed). These are not experiment runs.

Run order for 18 runs (alternate, one at a time): for round 1 to 3, for
type 1 to 3, run A then B in round 1 and 3, B then A in round 2. Record
work exit code, verify result, seconds and whether a commit exists. Do not
rerun failures. With n of 3 per cell and a 35B local model, run-to-run
noise alone can explain small differences.

## Recommendation

Drop nothing and adopt nothing on evidence yet, because no runs were made.
The simplified `worker-instructions.md` is cheap and keeps every
requirement, so it can be merged as plain editing. Keep the readability
rules advisory (`bin/lint --readability`) and out of the default lint:
23 + 26 findings remain in the protocol, task-format and README files and
the task files. Decide the per-kind thresholds after the experiment runs.
If it shows no difference, adopt the worker-instruction threshold only as a
style guide for new text, not as a gate.

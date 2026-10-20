# TF-039: Readability for worker-facing text

Question: does shorter, simpler worker-facing text measurably help a small
local model finish tasks? This branch measures the text, simplifies two
files, and runs 18 worker runs. Result: all 18 passed, so the experiment
shows no measurable difference between the original and simplified text.

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

## Experiment

Three tiny task types: (1) fix a return value so a test passes, (2) add a
function a test already calls, (3) fix gofmt and vet complaints. Two
conditions: A = original `worker-instructions.md` (from `main`) and verbose
task text; B = simplified instructions and short task text with the same
requirements and `Check:` lines. Three repetitions per cell, 18 runs, one at
a time, in a fresh scratch repo each, with pi and
`Ornith-1.5-35B-A3B-MLX-4bit`, `--timeout 8m`, then `taskfactory verify`.
Order: A then B in repetitions 1 and 3, B then A in repetition 2. Harness:
`docs/experiments/TF-039-run.sh`; texts: `docs/experiments/TF-039-fixtures/`.
No run was repeated or discarded. Invalid runs: 0, so each condition has 9
valid runs.

| Rep | Cond | Type | Work exit | Verify | Secs | Committed |
| --- | ---- | ---- | --------- | ------ | ---- | --------- |
| 1   | A    | 1    | 0         | PASS   | 66   | yes       |
| 1   | B    | 1    | 0         | PASS   | 51   | yes       |
| 1   | A    | 2    | 0         | PASS   | 25   | yes       |
| 1   | B    | 2    | 0         | PASS   | 27   | yes       |
| 1   | A    | 3    | 0         | PASS   | 31   | yes       |
| 1   | B    | 3    | 0         | PASS   | 22   | yes       |
| 2   | B    | 1    | 0         | PASS   | 29   | yes       |
| 2   | A    | 1    | 0         | PASS   | 29   | yes       |
| 2   | B    | 2    | 0         | PASS   | 40   | yes       |
| 2   | A    | 2    | 0         | PASS   | 35   | yes       |
| 2   | B    | 3    | 0         | PASS   | 185  | yes       |
| 2   | A    | 3    | 0         | PASS   | 36   | yes       |
| 3   | A    | 1    | 0         | PASS   | 24   | yes       |
| 3   | B    | 1    | 0         | PASS   | 34   | yes       |
| 3   | A    | 2    | 0         | PASS   | 36   | yes       |
| 3   | B    | 2    | 0         | PASS   | 37   | yes       |
| 3   | A    | 3    | 0         | PASS   | 51   | yes       |
| 3   | B    | 3    | 0         | PASS   | 60   | yes       |

Summary (pass rate, median wall seconds):

| Group  | A pass | A median | B pass | B median |
| ------ | ------ | -------- | ------ | -------- |
| Type 1 | 3/3    | 29       | 3/3    | 34       |
| Type 2 | 3/3    | 35       | 3/3    | 37       |
| Type 3 | 3/3    | 36       | 3/3    | 60       |
| All    | 9/9    | 35       | 9/9    | 37       |

Odd: one B run (rep 2, type 3) took 185 s against 22 to 60 s for its
siblings. Its cause was not investigated. Without it the B median is 35.

Conclusion: no measurable difference. Both conditions passed every run, so
the pass rate cannot separate them (a ceiling effect: the tasks are too easy
to fail). Median times differ by 2 s overall, well inside the run-to-run
spread (22 to 185 s). With n of 9 per condition, three per task type, only
a large effect on pass rate or time could show, and these tasks leave no
room for one. The data cannot say whether simpler text helps harder tasks,
longer contexts or weaker models.

## Recommendation

Adopt for worker instructions only, as a style guide, not a gate. The
simplified `worker-instructions.md` keeps every requirement and showed no
harm, but also no measurable gain, so do not claim a benefit. Keep the
rules advisory (`bin/lint --readability`) and out of the default lint:
23 MDS023, 26 MDS024 and 6 MDS036 findings remain in the protocol,
task-format and README files and the task files. Do not enable them
repo-wide. To learn more, rerun with harder tasks or a weaker model.

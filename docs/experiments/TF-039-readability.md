# TF-039: Readability for worker-facing text

Question: does shorter, simpler worker-facing text measurably help a small
local model finish tasks? This record has three parts: the advisory readability
counts, a detail check on the simplified text, and worker runs. The worker runs
are inconclusive. Both conditions passed every run on the 35B model, so the
pass rate cannot separate them. The 9B model timed out on every run, so it
gives no usable signal.

## Advisory check

`bin/lint --readability` reads `.mdsmith-readability.yml` and runs only MDS023
(paragraph-readability), MDS024 (paragraph-structure) and MDS036
(max-section-length) with the pinned mdsmith v0.57.0. It prints findings and a
`stats:` line, and always exits 0. The default `bin/lint` and `.mdsmith.yml` are
unchanged: `paragraph-readability: false` still holds for the whole repository.

Scope: `.taskfactory/tasks/inbox`, `.taskfactory/tasks/ready`,
`.taskfactory/tasks/active`, `.taskfactory/tasks/failed`,
`docs/worker-instructions.md`, `docs/protocol-v1.md`,
`docs/task-format-v1.md`, `skills/taskfactory/SKILL.md` and `README.md`.
Nothing under `.taskfactory/tasks/archive` is read.

## Thresholds per file kind

| File kind            | MDS023 max ARI | MDS024 sentences | MDS024 words | MDS036 lines |
| -------------------- | -------------- | ---------------- | ------------ | ------------ |
| Worker instructions  | 12.0           | 5                | 30           | 40           |
| Task files and specs | 14.0           | 6                | 40           | 60           |

Worker instructions are `docs/worker-instructions.md` and
`skills/taskfactory/SKILL.md`, which a small model reads before every task, so
they get the strict values. Task files, the protocol and task-format specs and
the README keep the mdsmith defaults for MDS023 and MDS024. MDS036 is 60 lines
for those kinds because 40 flagged ordinary sections that are mostly examples.

Caveat: ARI counts characters per word, so paragraphs full of paths and
commands score high even when the sentences are short.

## Counts per file kind

Counts are MDS023 / MDS024 / MDS036 findings, from
`mdsmith check -c .mdsmith-readability.yml` on the current tree. "Before" is
the file on `main`; files this task did not edit have the same count before
and after.

| File kind                   | Before      | After       |
| --------------------------- | ----------- | ----------- |
| .taskfactory/tasks/inbox    | 5 / 1 / 0   | 5 / 1 / 0   |
| .taskfactory/tasks/ready    | 4 / 4 / 1   | 4 / 4 / 1   |
| docs/worker-instructions.md | 4 / 5 / 0   | 0 / 0 / 1   |
| skills/taskfactory/SKILL.md | 2 / 0 / 1   | 0 / 0 / 1   |
| docs/protocol-v1.md         | 9 / 9 / 1   | 9 / 9 / 1   |
| docs/task-format-v1.md      | 3 / 9 / 2   | 3 / 9 / 2   |
| README.md                   | 6 / 10 / 3  | 6 / 10 / 3  |
| Total                       | 33 / 38 / 8 | 27 / 33 / 9 |

The "After" total is the `stats:` line from `bin/lint --readability`, which
reports MDS023=27 MDS024=33 MDS036=9 on the current tree. The remaining MDS036
finding in worker instructions is the single 45-line top-level section, one
line over the 40-line threshold. It is recorded, not hidden by changing the
threshold.

## Simplification

Files changed (prose only; no IDs, commands, `Check:` lines or exit codes):

- `docs/worker-instructions.md`: shorter sentences, paragraphs split, evidence
  paths in a list. The `tasks_dir` note and the `git.base_branch` wording are
  kept. The original is `main:docs/worker-instructions.md`.
- `skills/taskfactory/SKILL.md`: two paragraphs (the intro and the start of
  "Ready task contract") reworded. The skill keeps generic `tasks/` paths
  because it is embedded and must work with any `tasks_dir`.

Not changed: protocol, task-format, README and the task files.

## Detail check

`bin/detail-check` extracts backticked spans, `--flags`, task IDs, `exit N`
phrases and `Check:` spans from the original and simplified file, joins line
breaks first, and fails if an original item is missing from the simplified text.

```text
PASS docs/worker-instructions.md (vs main): 30 details kept
PASS skills/taskfactory/SKILL.md (vs main): 113 details kept
exit=0
```

Negative control: a copy of `main`'s worker instructions with `bin/lint`
replaced by `lint` and `TF-0` removed fails with `MISSING in simplified:
\`bin/lint\`` and `FAIL ... 1 of 30 details missing`, exit 1.

## Worker runs

Harness: `docs/experiments/TF-039-run.sh BINARY A|B 1|2|3|4|5`, with
`MODEL` selecting the model. Each trial runs in a fresh scratch repository
outside the checkout, with the pi adapter, `--timeout 8m`, then
`taskfactory verify`. Condition A uses `main`'s worker instructions with the
verbose task text. Condition B uses the simplified instructions with the short
task text. The two conditions differ in both files, as in the earlier run.

A run counts as a pass only when `work` exits 0, `verify` passes and the worker
made a commit. The harness's own verdict column is `verify` alone, and `verify`
checks the working tree, so it can pass a run whose worker timed out. Such runs
are counted as failures here.

Types 4 and 5 are harder: each needs several edits across files, and each has
one wrong test expectation the worker must correct.

- Type 4: rename `Parse` to `ParseInt` in all files and tests, and correct the
  expected value in `TestLoadLimit` from 7 to 5.
- Type 5: implement `Median`, add it to the `Summary` output, and correct
  `TestMedianEven` from 2.0 to 2.5.

### 35B model, types 4 and 5 (`Ornith-1.5-35B-A3B-MLX-4bit`)

Runs: 5 reps per cell, 4 cells, 20 runs total. Order alternates A/B by rep.
Zero invalid runs; none repeated or discarded.

| Cell      | Runs | Pass  | Median s | Min s | Max s |
| --------- | ---- | ----- | -------- | ----- | ----- |
| Type 4, A | 5    | 5/5   | 111      | 97    | 207   |
| Type 4, B | 5    | 5/5   | 139      | 89    | 272   |
| Type 5, A | 5    | 5/5   | 115      | 91    | 163   |
| Type 5, B | 5    | 5/5   | 123      | 71    | 175   |
| All A     | 10   | 10/10 | 113      | 91    | 207   |
| All B     | 10   | 10/10 | 124.5    | 71    | 272   |

Every run had work exit 0 and a commit. Both conditions reached the ceiling, so
the pass rate cannot show a difference between them.

### 9B model, `Ornith-1.5-9B-MLX-4bit`: no valid passes

Runs: 4 total. Type 4, condition B (smoke run) and type 1 condition A, type 1
condition B, type 3 condition A (grid, rep 1). Each ran to the 8 minute timeout:
work exit 1, `committed` no, and wall time 480 to 481 seconds. The
`verify` verdict was PASS on the uncommitted tree, which does not count. Valid
passes: 0 of 4. The grid was stopped after these runs by the coordinator, so
the cells have 1 run each (type 1 A and B, type 3 A) and the smoke run is
type 4 B. No cell has the five runs the design asks for, and no timing
comparison is possible.

### Earlier run, types 1 to 3, 35B model

From the earlier version of this record: 18 runs (3 types, both conditions, 3
reps), all 18 passed. Median seconds were 35 for condition A and 37 for
condition B. That run is at ceiling and says nothing about harder tasks.

## Conclusion

Inconclusive. At 35B, types 4 and 5 are at ceiling: 20 of 20 passed in both
conditions, so the simplified text neither helped nor hurt on these tasks.
Median times differ by about 11 seconds (113 against 124.5), within the spread
of the runs (71 to 272 seconds). At 9B, every run timed out, so the weaker-model
question is not measured. The simplified text is adopted as a style choice, not
a measured gain. The advisory readability check stays out of the default
`bin/lint` exit code.

Next step, if needed: a harder task family that the 35B model fails sometimes,
with five or more runs per cell, and a harness that counts a run as passing
only when a commit exists.

## Gate for worker-facing files (TF-115)

TF-115 gates the two worker-facing files in the default `bin/lint`:
`docs/worker-instructions.md` and `skills/taskfactory/SKILL.md`. Their MDS023,
MDS024 and MDS036 findings now fail the default lint, with the thresholds from
`.mdsmith-readability.yml`. Other Markdown stays advisory under
`bin/lint --readability`, and the rules still only report, so `--autofix`
leaves them alone. After the TF-115 prose edits both files have no findings
for the three rules. `bin/lint.readability.test.sh` covers the gate.

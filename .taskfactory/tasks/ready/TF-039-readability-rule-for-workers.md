# TF-039: Re-enable the mdsmith readability rule for worker context

## Goal

Decide, with measurements, whether simpler and more concise Markdown helps
workers and readers without losing detail, and give the repository an advisory
readability check that does not gate `bin/lint`.

## Dependencies

- TF-038

## Scope

TF-038 disabled the opinion rules in `.mdsmith.yml`. An earlier run of this
task exists on the unmerged branch `experiment/TF-039-readability` (worktree
`/Users/jesper/src/tf-readability`, last commit `2fd0b40`). It predates the
move of the task folders to `.taskfactory/tasks`, so port it onto the current
`main` instead of merging it: copy the advisory lint option, its config, its
test, the simplified worker instructions, the harness and the fixtures, then
fix every path.

Part 1, advisory check. `bin/lint --readability` runs only MDS023
(paragraph-readability), MDS024 (paragraph-structure) and MDS036
(max-section-length) from a separate config over the worker-facing files:
the non-archive task files under `.taskfactory/tasks/` (inbox, ready, active,
failed), `docs/worker-instructions.md`, `docs/task-format-v1.md`,
`docs/protocol-v1.md` and `skills/taskfactory/SKILL.md`. It prints findings and
a `stats:` line and always exits 0. Plain `bin/lint` is unchanged.

Part 2, concise prose without lost detail. Simplify
`docs/worker-instructions.md` and the skill text as the branch did, then prove
no detail was lost with a script, `bin/detail-check`, that extracts from the
original and the simplified file every backticked command, flag, path,
task ID, exit code and `Check:` line and fails when one is missing. Run it on
the pair. The skill file is embedded in the binary, so `bin/skill.test.sh` and
the embed test must still pass.

Part 3, a harder measurement. The earlier 18 runs all passed, so they could
not tell the texts apart. Repeat with a weaker model and harder tasks: use the
9B model (`omlx/Ornith-1.5-9B` as listed by the local omlx server; use the
largest local model that still fails some tasks if that name differs), at
least four task types of which two need several edits and a repaired test,
and at least five runs per cell and condition. Record pass rate, median
seconds and the number of runs. State plainly if the result is still
inconclusive.

Write the record to `docs/experiments/TF-039-readability.md`. It must list
MDS023, MDS024 and MDS036 finding counts per file kind, the threshold chosen
for each kind, the detail-check result and the comparison of a simplified task
against the original task with the same worker, including the number of runs.

## Constraints

Do not enable `paragraph-readability` or the related rules in `.mdsmith.yml`
for the whole repository, and keep their findings out of the `bin/lint` exit
code. Do not edit anything under `.taskfactory/tasks/archive/`. Edits to
measured files are prose only and must not change meaning, IDs, dependencies,
`Check:` lines or recorded evidence. Never touch git signing. Model runs use
scratch repositories outside the checkout. If a local model is unavailable,
say so in the record and mark Part 3 as not measured.

## Success criteria

### C1: The record lists each rule's counts

Check: grep -q MDS023 docs/experiments/TF-039-readability.md

### C2: The record lists the layout and size rules

Check: grep -q MDS036 docs/experiments/TF-039-readability.md

### C3: The record states a threshold per file kind

Check: grep -qi "threshold" docs/experiments/TF-039-readability.md

### C4: The record states the number of runs

Check: grep -qi "runs" docs/experiments/TF-039-readability.md

### C5: The repository-wide rule stays off

Check: grep -q "paragraph-readability: false" .mdsmith.yml

### C6: The archive is unchanged

Check: git diff --quiet main -- .taskfactory/tasks/archive

### C7: The readability option is advisory and exits 0

Check: bin/lint.readability.test.sh

### C8: No detail was lost in the simplified files

Check: bin/detail-check

### C9: The skill still passes its tests

Check: bin/skill.test.sh

### C10: Lint and task validation pass

Check: bin/lint && go run ./cmd/taskfactory validate .taskfactory/tasks

## Verification

Run each check from the repository root and report its exit code. Run
`bin/test` and `bin/lint` and report their exit codes. Paste the MDS023 count
line for each file kind and the detail-check output from the record. State the
number of worker runs per condition and whether the simplified text changed the
result, or say that the result was not measured.

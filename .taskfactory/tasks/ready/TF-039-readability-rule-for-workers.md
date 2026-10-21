# TF-039: Re-enable the mdsmith readability rule for worker context

## Goal

Measure whether shorter sentences and simpler paragraphs help workers, then
choose a per-file-kind readability threshold for mdsmith rule MDS023
(paragraph-readability), without enabling the rule repo-wide.

## Dependencies

- TF-038

## Scope

TF-038 disabled the opinion rules in `.mdsmith.yml`. Measure the three
readability rules on the worker-facing files only: the non-archive task files
under
`tasks/inbox/`, `tasks/ready/`, `tasks/active/` and `tasks/failed/`,
`docs/worker-instructions.md`, `docs/task-format-v1.md` and
`docs/protocol-v1.md`. Run `mdsmith help rule MDS023` and the rules MDS024
(paragraph-structure) and MDS036 (max-section-length) for their settings.

Write the record to `docs/experiments/TF-039-readability.md`. It must list
MDS023, MDS024 and MDS036 finding counts per file kind, the threshold chosen
for each kind, and one comparison of a simplified task against the original
task with the same worker, including the number of runs.

## Constraints

Do not enable `paragraph-readability` or the related rules in `.mdsmith.yml`
for the whole repository, and keep their findings out of the `bin/lint` exit
code until every file in the measured scope meets its threshold. Rewording
existing archived tasks is out of scope: do not edit anything under
`tasks/archive/`. Edits to the measured files are prose only and must not
change meaning, IDs, dependencies, `Check:` lines or recorded evidence.

## Success criteria

### C1: The record lists each rule's counts

Check: grep -q MDS023 docs/experiments/TF-039-readability.md && grep -q MDS036 docs/experiments/TF-039-readability.md

### C2: The record states a threshold per file kind

Check: grep -qi "threshold" docs/experiments/TF-039-readability.md

### C3: The repository-wide rule stays off

Check: grep -q "paragraph-readability: false" .mdsmith.yml

### C4: The archive is unchanged

Check: git diff --quiet main -- tasks/archive

### C5: Lint and task validation pass

Check: bin/lint && go run ./cmd/taskfactory validate tasks

## Verification

Run each check from the repository root and report its exit code. Run `bin/test`
and `bin/lint` and report their exit codes. Paste the MDS023 count line for each
file kind from the record. State the number of worker runs in the comparison
and whether the simplified task changed the result, or say that the result was
not measured.

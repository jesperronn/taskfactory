# TF-111: Explore evidence-based automatic worker selection

## Goal

Write down which recorded evidence would justify automatic worker selection,
and whether TaskFactory should build it. This task produces a document only.

## Dependencies

- TF-048
- TF-049
- TF-056

## Scope

Create `docs/worker-selection.md`. Read the evidence format in
`docs/task-format-v1.md` (the attempt schema) and the adapter and model
fields in `docs/local-workers-v1.md`. Use the fields adapter, model, outcome,
attempts and duration as the data to judge. Write the document with the
section headings named in the success criteria. No code changes.

## Constraints

Worker selection stays manual in v1. Do not change any Go, shell or config
file. Make no claim the recorded data cannot support. Mark each open question
as open.

## Success criteria

### C1: The document has a question section

Check: grep -q '^## Question$' docs/worker-selection.md

### C2: The document has a recorded data section

Check: grep -q '^## Recorded data$' docs/worker-selection.md

### C3: The document has an evidence threshold section

Check: grep -q '^## Evidence threshold$' docs/worker-selection.md

### C4: The document ends with a recommendation section

Check: grep -q '^## Recommendation$' docs/worker-selection.md

### C5: Lint passes on the new document

Check: bin/lint docs/worker-selection.md

## Verification

Run each check from the repository root and report each exit code. State
the recommendation in one sentence in the report.

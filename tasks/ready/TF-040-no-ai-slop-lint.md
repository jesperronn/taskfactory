# TF-040: Lint for AI-generated prose tells

## Goal

Keep the non-archive task files and project documentation free of mechanical
AI-style phrasing, using mdsmith rules MDS055 and MDS056, while the wording
of every task contract stays exactly as specified.

## Dependencies

- TF-038

## Scope

mdsmith v0.57.0 has no `no-llm-tells` convention. Enable the two rules
directly. MDS056 (forbidden-text) takes a `contains:` list and MDS055
(forbidden-paragraph-starts) takes a `starts:` list; `mdsmith help rule
MDS056` and `mdsmith help rule MDS055` show the settings. Put the lists in a
separate advisory config, `.mdsmith-tells.yml`, so `.mdsmith.yml` and the
`bin/lint` exit code do not change. Check the merge with `mdsmith kinds
resolve FILE`.

Add a `--tells` mode to `bin/lint` that runs the advisory config over
`docs/`, `README.md`, `tasks/inbox/`, `tasks/ready/`, `tasks/active/` and
`tasks/failed/`. It prints the findings and a line `stats: tells MDS055=N
MDS056=N`, and it exits 0.

Review every finding by hand and classify it as a true tell to reword, a false
positive to allow for, or a domain term the project keeps. Record the counts
and the classification totals in `docs/experiments/TF-040-tells.md` with the
lines `MDS055: N`, `MDS056: N` and `unclassified: 0`.

`taskfactory validate` does not check tasks under `tasks/archive/`; that gap is
covered by TF-053 and is context only. This task does not depend on it.

## Constraints

Archived tasks are out of scope: do not edit anything under `tasks/archive/`,
and rewording existing archived tasks is not allowed. Reword only prose in the
in-scope files. Never change task IDs, dependencies, `Check:` lines, exit
codes, specifications or acceptance criteria. Do not weaken other lint rules or
change `.mdsmith.yml`. Keep the tell findings out of the default `bin/lint`
exit code until the in-scope backlog is clean.

## Success criteria

### C1: The advisory config names both rules

Check: grep -q forbidden-text .mdsmith-tells.yml

### C2: The advisory mode reports and exits 0

Check: bin/lint --tells > /dev/null 2>&1

### C3: The record has counts and no unclassified findings

Check: grep -q "unclassified: 0" docs/experiments/TF-040-tells.md

### C4: The archive is unchanged

Check: git diff --quiet main -- tasks/archive

### C5: The default lint and task validation pass

Check: bin/lint && go run ./cmd/taskfactory validate tasks

## Verification

Run each check from the repository root and report its exit code. Run `bin/test`
and `bin/lint` and report their exit codes. Paste the `stats: tells` line and
the classification totals from the record. Commit the reworded prose in one
commit and confirm `taskfactory validate tasks` still passes after each batch
of edits.

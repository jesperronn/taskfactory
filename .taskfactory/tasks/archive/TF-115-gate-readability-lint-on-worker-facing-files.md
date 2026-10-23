# TF-115: Gate readability lint on worker-facing files

## Goal

TF-039 added the advisory `bin/lint --readability`. Make the three readability
rules (MDS023, MDS024, MDS036) part of the default `bin/lint` for the two
files a small model reads before every task: `docs/worker-instructions.md`
and `skills/taskfactory/SKILL.md`. Other Markdown stays advisory. The rules
only report and cannot be autofixed, so `bin/lint --autofix` leaves them
alone.

## Dependencies

None

## Scope

- Fix the current findings in the two files by editing prose only. Run
  `bin/detail-check` to prove no command, flag, path, ID or `Check:` line is
  lost. The strict thresholds from `.mdsmith-readability.yml` apply.
- Make the default `bin/lint` run the three rules on those two files with the
  same thresholds, so a finding exits non-zero. Keep one source of
  thresholds, not two copies.
- Add a case to `bin/lint.readability.test.sh` that shows a long sentence in
  a worker-facing file fails the default lint, and a long sentence in another
  Markdown file does not.
- Note in `docs/experiments/TF-039-readability.md` that the two files are now
  gated.

## Constraints

Do not enable the rules repo-wide, and do not edit the archive. Prose edits
keep IDs, dependencies and `Check:` lines. Do not touch commit signing.

## Success criteria

### C1: The gated files have no readability findings

Check: bin/lint docs/worker-instructions.md skills/taskfactory/SKILL.md

### C2: No detail was lost

Check: bin/detail-check

### C3: The gate is tested

Check: bin/lint.readability.test.sh

### C4: The skill still installs and reads correctly

Check: bin/skill.test.sh

### C5: The full test suite passes

Check: bin/test

## Verification

Run each check from the repository root and report each exit code.

## Claim

Owner: haiku-worker-115
Branch: task/TF-115
Worktree: /Users/jesper/src/taskfactory/.taskfactory/worktrees/TF-115
Base commit: a9243b4a5c938b60a28d9488d0650ed3b1ca3fb3
Started at: 2026-10-23T04:23:37Z

# TF-062: Quick start must state the Dependencies format

## Goal

A newcomer who writes `None.` under Dependencies must not pass `validate`
on an inbox file and then be refused by `promote`.

## Scope

Either make `validate` apply the same Dependencies rule for inbox files as
`promote` does, or have the README quick start step 3 say Dependencies
takes `None` or `- TF-NNN` lines exactly. Prefer both.

## Notes

Found by the verbatim quick start rerun (docs/experiments/
quickstart-verbatim-rerun.md). `validate` printed "1 task file(s) valid";
`promote` then failed with "Dependencies: use None or - TF-NNN lines".

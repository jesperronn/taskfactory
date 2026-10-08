# TF-039: Re-enable the mdsmith readability rule for worker context

## Goal

Make task files and worker-facing docs easier for workers, especially small
local models, to read and act on, by re-enabling mdsmith rule MDS023 (paragraph
readability) once mdsmith is the Markdown linter.

## Dependencies

- TF-038 must conclude with mdsmith adopted and `.mdsmith.yml` in place.

## Scope

TF-038 deliberately disables the opinion rules (MDS023 paragraph readability,
MDS022 file length, MDS018 emphasis-as-heading) so mdsmith matches Prettier's
formatting-only behavior. A trial run of mdsmith v0.57.0 with default settings
reported 25 MDS023 findings across `docs/*.md` and `README.md`, with an index
above its default threshold of 14. Revisit that rule later with a different
purpose: shorter sentences and simpler paragraphs may reduce the context and
reasoning a worker needs. Measure first. Pick the files that workers actually
read (task files, `docs/worker-instructions.md`, protocol and format docs),
choose a threshold per file kind, and compare worker results on a simplified
task against the original. Consider the related rules MDS024 (paragraph
structure), MDS028 (token budget) and MDS036 (section length) as well.

## Constraints

Do not enable the rule repo-wide without measuring benefit. Keep it out of
`bin/lint` failures until the existing docs conform or a per-file threshold is
agreed. Do not reword specifications in ways that change meaning. Archived tasks
may be simplified too, as decided by the owner, under the same limits as TF-040:
prose only, no change to IDs, dependencies, `Check:` lines or evidence, and in a
separate commit.

## Notes

Origin: roadmap suggestion from the project owner while reviewing the TF-038
mdsmith trial. Unrefined; promote to ready only after TF-038 lands and the
experiment shows a measurable effect on worker success.

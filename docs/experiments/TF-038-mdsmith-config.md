# TF-038 mdsmith configuration experiment

Pinned tools: `github.com/jeduden/mdsmith/cmd/mdsmith@v0.57.0` (run with
`go run`), Prettier 3.6.2 (used for the comparison before it was removed), and
goldmark v1.7.13 with the GFM extension (HTML comparison helper, built in a
temporary module outside the repository).

## Decision

**Switch. `bin/lint` now runs mdsmith v0.57.0 in place of Prettier, and the
switch passes the rendered-HTML safety check on every fixture and on all 47
tracked Markdown files.**

The project owner chose the `github` convention and the rule set below. Two
rules had to be changed from the task text because they changed the rendered
HTML:

- `no-trailing-spaces` is disabled. Its fix removes the two-space hard line
  breaks, which changes the rendered `<br>` elements (fixture `07`, and
  `docs/TASKFACTORY-SPEC.md` in the repository run).
- `list-indent` is set to 3 spaces, not 2. At 2 spaces, the fix moves nested
  bullets out of their numbered item, which changes list structure (fixture
  `08`, and step 6 of `docs/protocol-v1.md`). Three spaces match the width of a
  numbered marker, and the nested bullets stay inside the item.

Other deviations from the task text are listed under "Differences from
Prettier" below. Each one is a rule that mdsmith cannot enforce as written,
not a weakened check.

## Safety check

The check is run on copies of the files under `mktemp -d`. Nothing in the
worktree was fixed until the copies passed.

1. Run `mdsmith fix` on a copy and then `mdsmith check` on the result. The check
   must exit 0.
2. Render the original and the fixed file with goldmark (GFM extension) and
   compare the HTML two ways:
   - STRICT: the HTML is byte-identical.
   - NORMALIZED: whitespace runs are collapsed to a single space, and spaces
     next to tags are removed. Soft line wraps are whitespace-only changes, so
     they pass this comparison. Hard breaks (`<br>`), tags, list nesting and
     text content must match.
3. The gate is NORMALIZED. A file that fails it is a blocker.

## Convention and layering

`.mdsmith.yml` sets `convention: github`. The convention is a base layer:
mdsmith applies defaults first, then the convention, then the top-level
`rules:`, then kinds and overrides. `mdsmith kinds resolve docs/protocol-v1.md`
confirms the winning source for each rule:

- `markdown-flavor`: `gfm`, from `convention.github`
- `no-inline-html`: allow `details` and `summary`, from `convention.github`
- `emphasis-style`: bold `asterisk`, italic `underscore`, from the convention
  (the user rules repeat the same values)
- `list-marker-style`: `dash`, from the user rules
- `no-trailing-spaces`: `false`, from the user rules (overrides the default)

The `portable` convention was not evaluated, as the owner directed. One early
fixture run under `portable` flagged the GFM table in fixture `01` as
unfixable, because `portable` pins the `commonmark` flavor.

## Final rule settings

| Rule                                                                                                                                                                  | Setting                                                   | Reason                                                      |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- | ----------------------------------------------------------- |
| paragraph-readability (MDS023), max-file-length (MDS022), no-emphasis-as-heading (MDS018), table-readability (MDS026)                                                 | off                                                       | opinion rules, not formatting                               |
| other opinion rules (paragraph-structure, conciseness-scoring, token-budget, max-section-length, first-line-heading, over-repetition, duplicated-content, occurrence) | off                                                       | opinion rules, not formatting                               |
| line-length (MDS001)                                                                                                                                                  | max 80, `reflow: true`, `stern: true`, `heading-max: 120` | see "Differences from Prettier"                             |
| table-format (MDS025)                                                                                                                                                 | `style: consistent`, `separator-style: spaced`            | matches the Prettier table layout                           |
| list-indent (MDS016)                                                                                                                                                  | `spaces: 3`                                               | keeps numbered-item children inside the item (see Decision) |
| no-trailing-spaces (MDS006)                                                                                                                                           | off                                                       | its fix removes hard line breaks (see Decision)             |
| blank-line-around-headings / lists / fenced-code (MDS013/014/015)                                                                                                     | on                                                        | matches the Prettier layout                                 |
| list-marker-style (MDS045)                                                                                                                                            | `dash`                                                    | from the convention; matches Prettier                       |
| emphasis-style (MDS042)                                                                                                                                               | bold `asterisk`, italic `underscore`                      | from the convention; matches Prettier                       |

`ignore:` excludes `testdata/markdown/**`, which holds the deliberately
misformatted fixtures.

## Fixture results

Fixtures are in `testdata/markdown/`. "Prettier" is the Prettier 3.6.2 check on
the original file. "mdsmith check" is the final config on the original file.
"Fix" is the final config's `mdsmith fix` followed by `mdsmith check`. "HTML"
is the NORMALIZED result, with STRICT in parentheses. "= Prettier" compares the
fix output with `prettier --write --prose-wrap always`.

| Fixture                  | Prettier | mdsmith check | Fix, then check | HTML                  | = Prettier |
| ------------------------ | -------- | ------------- | --------------- | --------------------- | ---------- |
| 01-table-misaligned      | fail     | fail          | exit 0          | SAME (SAME)           | no         |
| 02-star-bullets          | fail     | fail          | exit 0          | SAME (SAME)           | yes        |
| 03-emphasis-star         | fail     | fail          | exit 0          | SAME (SAME)           | yes        |
| 04-long-lines            | fail     | fail          | exit 0          | SAME (differs, wraps) | yes        |
| 05-short-wrapped         | fail     | pass          | unchanged       | SAME (SAME)           | no         |
| 06-nested-list-3space    | fail     | pass          | unchanged       | SAME (SAME)           | no         |
| 07-hard-breaks           | pass     | pass          | unchanged       | SAME (SAME)           | yes        |
| 08-nested-under-numbered | pass     | pass          | unchanged       | SAME (SAME)           | yes        |
| 09-clean                 | pass     | pass          | unchanged       | SAME (SAME)           | yes        |

Fixture `04` is STRICT-different only because Prettier and mdsmith both rewrap
the paragraph, which changes soft line breaks. The NORMALIZED comparison
accepts this.

Earlier runs, which are the reason for the two rule changes:

| Config                                                     | Fixture 07 (hard breaks) | Fixture 08 (numbered + nested) |
| ---------------------------------------------------------- | ------------------------ | ------------------------------ |
| `convention: github`, `list-indent: 2`, trailing spaces on | DIFFERS                  | DIFFERS                        |
| `list-indent: 3`, trailing spaces on                       | DIFFERS                  | SAME                           |
| `list-indent: 2`, trailing spaces off                      | SAME                     | DIFFERS                        |
| final (`list-indent: 3`, trailing spaces off)              | SAME                     | SAME                           |

Under the earlier convention-less config, fixture `01` also passed the gate,
and fixtures `02`, `03`, `05`, `06` and `09` did too.

## Repository results

Scope: the 47 tracked Markdown files, copied to a temporary directory.

| Config                                      | Check fails before fix | Files changed by fix | HTML damaged (NORMALIZED)                            |
| ------------------------------------------- | ---------------------- | -------------------- | ---------------------------------------------------- |
| `convention: github`, 2-space, trailing on  | 20                     | 2                    | 2: `docs/protocol-v1.md`, `docs/TASKFACTORY-SPEC.md` |
| `convention: github`, 3-space, trailing on  | 19                     | 1                    | 1: `docs/TASKFACTORY-SPEC.md`                        |
| `convention: github`, 2-space, trailing off | 19                     | 1                    | 1: `docs/protocol-v1.md`                             |
| `convention: github`, 3-space, trailing off | 18                     | 0                    | 0                                                    |
| final config                                | 0                      | 0                    | 0                                                    |

The final config passes `mdsmith check` on all 47 files with no fix needed.
Prettier also changed none of these files. The original repository was already
Prettier-clean, so the mechanical reformat is empty, and no reformat commit is
made.

The 18 failures in the intermediate runs were MDS001 headings over 80
characters and a table cell that tripped MDS026. The final config handles these
as described under "Differences from Prettier".

## Differences from Prettier

Prettier was the previous gate. These are the differences that remain after the
switch:

- **Under-wrapped paragraphs are not joined.** mdsmith `reflow` only shortens
  long lines. Fixture `05` passes mdsmith check and keeps its short lines.
  Prettier joins them.
- **Headings are not reflowed.** mdsmith cannot wrap a heading. Prettier never
  checked heading length, so `heading-max: 120` keeps the repository's long
  task headings passing. Headings over 120 characters now fail.
- **Unbreakable lines are not flagged.** `stern: true` flags only lines with a
  breakable space past 80. A long code span or URL is not an error.
- **Trailing spaces are not stripped.** The trailing-space rule is off, so a
  stray trailing space that is not a hard break is not reported. Prettier
  removes it.
- **Bullets nested under bullets use 3 spaces, not 2.** Prettier uses 2, and
  fixture `06` is the one case that differs. The repository has no such case.
- **Table padding differs for aligned columns.** Prettier and mdsmith both
  align tables. In fixture `01` the centered column is padded differently.
- **`table-readability` (MDS026) is off.** It is an opinion rule. It was
  flagging one table in `docs/experiments/TF-015-worker-comparison.md`.
- **mdsmith scans installed packages.** `mdsmith check` with no arguments also
  walks `node_modules` if it exists. Earlier it reported 419 failures from
  installed packages. Node was removed, so `bin/lint` no longer runs in that
  state. A developer who installs Node packages will see those failures.

## Items not changed

- `.github/dependabot.yml` does not exist in this worktree, so no npm entry was
  removed. Task TF-036 (ready) still asks for an npm update entry. That entry
  would have no `package.json` to track, so TF-036 needs revisiting.
- `tasks/ready/TF-036-dependabot.md`, `tasks/ready/TF-030-lint-go-vet-paths.md`
  and `docs/experiments/TF-015-worker-comparison.md` mention Prettier. Those are
  task contracts and a historical record, which this task does not edit.
- `docs/TASKFACTORY-SPEC.md` has a generic `Node:` example in a table of
  project commands. It is not a Prettier or npm reference.

## Verification

Commands are recorded with exit codes in the worker report. The final
verification run is:

- `go run github.com/jeduden/mdsmith/cmd/mdsmith@v0.57.0 check`: exit 0,
  47 files checked, 0 failures.
- `bin/lint`: see the worker report.
- `bin/test`: see the worker report.
- `go run ./cmd/taskfactory validate`: see the worker report.

# TF-037 Markdown tooling evaluation

Status: historical. This evaluation led to keeping Prettier at the time; the
project later adopted mdsmith (TF-038), so its decision no longer applies.
Salvaged from the unmerged branch `task/TF-037-go-markdown-lint`.

Decision: keep Prettier 3.6.2 in `bin/lint`. Neither candidate can check and fix
this repository's Markdown the way `prettier --check --prose-wrap always` does,
so no switch was made and Node, `package.json`, `package-lock.json` and the CI
Node steps remain.

All candidate runs used throwaway copies of the 47 tracked `*.md` files under
`mktemp -d`. The worktree was not used as the cwd for any candidate run.

## Versions and commands

| Tool                | Pinned version                                                       | Command                                                                                          |
| ------------------- | -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Prettier (baseline) | 3.6.2 (`node_modules/.bin/prettier --version`)                       | `prettier --check --prose-wrap always '**/*.md'`                                                 |
| mdsmith             | v0.57.0 (`mdsmith version`)                                          | `go run github.com/jeduden/mdsmith/cmd/mdsmith@v0.57.0 check\|fix <files>`                       |
| mdreflow            | v0.3.0 (`mdreflow -version` reports `dev`; version pinned by module) | `go run github.com/jbeda/mdreflow/cmd/mdreflow@v0.3.0 --check -mode wrap --max-width 80 <files>` |

Latest mdsmith release tags were `v0.57.0` (stable) and `v0.58.0-rc.*`
(prerelease); v0.57.0 was chosen. mdreflow v0.3.0 is the latest tag listed by
`go list -m -versions`.

Baseline in the worktree: `bin/lint` passes with Prettier 3.6.2 and reports "All
matched files use Prettier code style!".

## Results

### mdsmith v0.57.0

- Check mode: yes (`check`). Fix mode: yes (`fix`), with `--build-*` flags for
  build targets.
- Default configuration on the 47 files: `check` exits 1 with 111 findings:
  MDS001 line-length 32, MDS023 paragraph-readability 64, MDS006 trailing spaces
  3, MDS016 5, MDS018 3, MDS022 3, MDS026 1.
- `fix` (default config) changes 2 files and leaves 101 findings unfixed.
- With `paragraph-readability: false` and `line-length: {max: 80, reflow: true}`
  in a throwaway `.mdsmith.yml`, `check` still reports 47 findings (MDS001 32,
  MDS006 3, MDS016 5, MDS018 3, MDS022 3, MDS026 1). `fix` changes 2 files (17
  diff lines) and the findings remain.
- Cannot reproduce Prettier's output: the repository's own, already-Prettier
  files are flagged by its default rules.
- Probe file (table with misaligned columns, `*` list markers, `*emphasis*`,
  trailing spaces): mdsmith flagged MDS006 trailing spaces and MDS025 table
  formatting only; it did not flag list marker or emphasis style, which Prettier
  does.

### mdreflow v0.3.0

- Check mode: yes (`--check`, `--diff`). Fix mode: yes (in-place when given
  paths).
- It is a prose reflow tool, not a linter: it only changes where lines break
  inside paragraphs and never reports table, list-marker, or emphasis issues.
- Sentence mode (default) `--check` on the 47 files: exit 1 and 47 lines of
  "would reformat" output (sentence-per-line is not the repository convention).
- Wrap mode at 80 `--check`: exit 1, 15 files would reformat. Applying
  `-mode wrap --max-width 80` to a copy changed 14 files (279 changed diff
  lines). Prettier `--check` on that result still reports 14 files, so
  mdreflow's wrap output does not match Prettier's.
- Sample diff (`docs/config-v1.md`, wrap mode): mdreflow moves `and says to run`
  up onto the previous line; Prettier keeps the original break:

  ```diff
  -   `.taskfactory/config.toml` is absent. The error identifies the missing path
  -   and says to run `taskfactory init` from the repository.
  +   `.taskfactory/config.toml` is absent. The error identifies the missing path and
  +   says to run `taskfactory init` from the repository.
  ```

  The left-hand side is the repository's Prettier-accepted form.

- Probe file: mdreflow `--check -mode wrap --max-width 80` exit 0 while
  `prettier --check` exit 1 on the same file.

## Why no switch

Task criteria require a tool that can check and fix this Markdown with a small,
reviewable change. Neither candidate meets that:

1. mdreflow cannot check the non-prose formatting that `bin/lint` currently
   enforces (tables, list markers, emphasis, trailing spaces), so replacing
   Prettier with it would weaken lint strictness.
2. mdreflow wrap output differs from Prettier at 14 files / 279 lines, so the
   switch would be a large reformat, not a mechanical one.
3. mdsmith reports 47 findings after the best configuration tried and its `fix`
   resolves only 2 files; the remaining findings would need manual rewrapping
   across many files.

No `bin/lint`, `bin/lint.test.sh`, `bin/lint.real-prettier.test.sh`, CI, Node,
or documentation changes were made.

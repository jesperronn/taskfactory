# TF-038: Configure mdsmith as the Markdown checker and fixer

## Goal

Decide, with evidence, whether a configured mdsmith can replace Prettier in
`bin/lint` with working check and autofix, and switch only if it can do so
safely.

## Dependencies

- TF-005

## Scope

Pin `github.com/jeduden/mdsmith/cmd/mdsmith@v0.57.0` (run with `go run`). Write
a project `.mdsmith.yml` that disables opinion rules (readability, file length,
emphasis-as-heading, and similar) and enables formatting rules: line length 80
with reflow, table format, list indent, list marker style, emphasis style, no
trailing spaces, blank lines around headings, lists and fences. Read
`mdsmith help rule <id>` for options. Build a fixture directory under
`testdata/markdown/` with deliberately misformatted files (misaligned table, `*`
bullets, `*em*`, over-long lines, wrapped lines that are too short, 3-space
nested list indent, trailing double-space hard breaks, nested lists under
numbered items). For each fixture record whether Prettier 3.6.2 and mdsmith
check flag it, and whether mdsmith fix produces output that mdsmith check
accepts. Verify that fix never changes rendered meaning: render before and after
with goldmark (a small Go test or `go run` helper) and compare HTML; hard breaks
and list nesting must be preserved. Also compare mdsmith fix output with
`prettier --write --prose-wrap always` output on the fixtures and the repository
Markdown, and count differing files. Write the evidence to
`docs/experiments/TF-038-mdsmith-config.md`.

If every safety check passes and the repository's own Markdown passes
`mdsmith check` with at most a small, reviewable set of mechanical changes, then
switch `bin/lint` (check mode, `--autofix` runs fix, keep exit codes 0, 1, 2),
update its tests, and remove Prettier, `package.json`, `package-lock.json`, the
Node CI steps and the npm Dependabot entry, with the mechanical reformat in its
own commit. Otherwise change nothing but the experiment record and list the
blocking gaps.

## Constraints

Do not weaken checks to pass. Do not run any fix on the real repository files
until the fixture safety checks pass; work on copies under `mktemp -d`. Never
run `bin/lint --autofix` on unrelated files. A fix that changes rendered output
fails the task's safety check.

## Success criteria

### C1: The mdsmith config and fixtures exist

Check: test -s .mdsmith.yml && test -d testdata/markdown

### C2: The evidence record exists with a clear decision

Check: grep -n -i "decision" docs/experiments/TF-038-mdsmith-config.md

### C3: Repository checks and task validation pass

Check: bin/test

## Verification

Run `bin/test`, `bin/lint` and `go run ./cmd/taskfactory validate`. If the
switch happened also run `bin/lint --autofix` only on a fixture copy and show it
exit 0 afterwards. Report each exit code, the HTML comparison result and whether
the switch was made.

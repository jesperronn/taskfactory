# TF-005: Add the repository lint script

## Goal

Create an executable `bin/lint` that checks Markdown formatting and offers an
explicit fix mode.

## Dependencies

None

## Scope

Add `bin/lint`. With no arguments, run Prettier in check mode over `**/*.md`
with `--prose-wrap always`, and return its exit status. With `--autofix`, run
`npx prettier --write --prose-wrap always "**/*.md"`. Reject unsupported
arguments with a short usage message and nonzero exit status. Include clearly
marked Bash and Go lint placeholders; they must not invoke linters yet.

## Constraints

Keep the script small, runnable from any working directory, and scoped to this
repository. Do not add package dependencies or modify existing Markdown files.
Preserve the current uncommitted specification changes.

## Success criteria

- `bin/lint` is executable and invokes the Markdown check command.
- `bin/lint --autofix` invokes the specified Markdown write command.
- Unsupported arguments fail clearly.
- Bash and Go lint placeholders are present without side effects.

## Verification

Use a temporary mock `npx` to verify arguments and exit status in both modes,
then check shell syntax and executable permissions. Report changed files and
verification results. Do not run the real write command over the repository.

## Outcome

Implemented in `bin/lint`. Verified executable permission, `bash -n`, both
Prettier argument lists with a temporary `npx` mock, repository-root execution,
invalid argument handling, and linter exit-status propagation. The real write
command was not run over repository Markdown.

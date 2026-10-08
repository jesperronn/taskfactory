# Task file format v1

This document defines the on-disk contract for executable tasks. V1 uses
restricted Markdown so people can review and edit task contracts without a
separate authoring tool. The validator recognizes only the syntax below; it does
not interpret general Markdown, HTML, front matter, or formatting extensions.
This keeps task parsing in the standard library and adds no Go dependency. TOML
remains the format for project policy in `.taskfactory/config.toml`.

## File and identity rules

A task file name is `<ID>-<slug>.md`. An ID matches `TF-[0-9]{3}` and is unique
across all five task directories. A slug is one or more lowercase ASCII letters,
digits, or single hyphens; it cannot begin or end with a hyphen. The `ID` in the
first heading must exactly equal the filename ID. The title after the colon is
non-empty, single-line text. The slug is not derived from or compared with the
title.

V1 task files use UTF-8, LF line endings, and no tabs. The first line is the
task heading. Headings and their order are exact; blank lines may separate
blocks. Lists use `- ` and each list item occupies one line. Text fields may
wrap across lines until the next heading or list item. Inline HTML, nested
lists, tables, links, and fenced code blocks are not part of the task contract.
Verification commands are single-line values and are not parsed as shell syntax
by the task validator.

## Ready task fields

A ready task contains these headings exactly once and in this order:

1. `# <ID>: <title>`
2. `## Goal`
3. `## Dependencies`
4. `## Scope`
5. `## Constraints`
6. `## Success criteria`
7. `## Verification`

All seven are required. Goal, scope, and constraints each contain non-empty
prose. Dependencies contains either the single line `None` or one or more unique
IDs, each as `- TF-NNN`. Dependencies may not refer to the task itself. Every
referenced ID must exist in exactly one task file. A dependency is resolved only
when that task is in `tasks/archive/`; files in every other directory leave it
unresolved. Dependency cycles are invalid.

Success criteria are one or more consecutive pairs. Each criterion heading has
the exact form `### CN: <text>`, where `N` is a positive decimal sequence number
and `<text>` is non-empty single-line text. Criterion IDs begin at C1 and
increase by one without gaps or duplicates. The next non-empty line must have
the exact form `Check: <command>`, where the command is non-empty and
single-line. Blank lines may separate the heading and its `Check:` line, but no
other heading or text may intervene. Each criterion has exactly one check;
checks cannot appear outside a criterion pair. This structure pairs each
criterion with its verification and makes the pair machine-readable. Commands
are run from the task's project worktree root, in listed order, as configured
command strings. The v1 task format does not prescribe shell expansion or
command splitting; execution policy belongs to the worker/orchestrator
implementation.

`## Verification` contains task-level reporting instructions as a non-empty
prose field. It does not replace criterion checks. A worker records the exact
command and exit status for every check in attempt evidence.

## Lifecycle metadata and evidence

Claim moves the task file atomically from `tasks/ready/` to `tasks/active/` and
appends this required block after `## Verification`:

```text
## Claim

Owner: <non-empty worker identifier>
Branch: <non-empty branch name>
Worktree: <canonical absolute path>
Base commit: <full Git commit object ID>
Started at: <RFC 3339 UTC timestamp>
```

Each key appears exactly once. `Worktree` is an absolute, normalized path with
no `.` or `..` components or trailing separator (except for the filesystem
root). Resolve it and `git.worktree_root` from `.taskfactory/config.toml` to
canonical absolute paths before validation. A relative configured root is
resolved from the project root. `Worktree` must equal the resolved root joined
with the task ID as one path component (`<git.worktree_root>/<ID>`). The
configured root may be outside the repository; the task format does not require
a worktree to be inside the repository. A task in `tasks/ready/` must not have a
Claim block. A task in `tasks/active/` must have one. When a claimed task moves
to `tasks/failed/` or `tasks/archive/`, its Claim block is retained unchanged;
it is not rewritten to represent later events. A new active-to-archive
transition requires that retained Claim block. Static validation of an existing
task in `tasks/archive/` permits the Claim block to be absent, preserving legacy
archived records created before this format; missing metadata is not inferred or
fabricated. If an archive Claim block is present, it must be complete and valid.

Each attempt is recorded by appending one JSON object and LF to
`.taskfactory/evidence/<ID>.jsonl`. This file is append-only: existing bytes
must never be changed or removed. It is created on first attempt. Each line is
one UTF-8 JSON object with exactly these keys:

```json
{
  "task_id": "TF-123",
  "attempt": 1,
  "recorded_at": "2026-10-08T12:00:00Z",
  "outcome": "FAILED",
  "branch": "feature/TF-123",
  "base_commit": "0123456789abcdef0123456789abcdef01234567",
  "result_commit": "",
  "checks": [{ "criterion": "C1", "command": "go test ./...", "exit_code": 1 }],
  "note": "test failed"
}
```

`task_id` matches the task; `attempt` is a positive integer, consecutive per
task; `recorded_at` is an RFC 3339 UTC timestamp; `outcome` is `PASS`, `FAILED`,
or `BLOCKED`; commits are full lowercase hexadecimal Git object IDs (40 or 64
characters; result may be empty before a commit exists); `checks` records every
criterion check in criterion order with its exact command and integer exit code;
`note` is a string and may be empty. Repeated attempts append new lines with the
next attempt number. A successful worker report alone does not archive a task:
integration must succeed before the task moves to `tasks/archive/`. Evidence
records worker attempts and does not replace integration evidence required by
the protocol.

## Complete valid ready task

```markdown
# TF-123: Add health command

## Goal

Expose a health command that reports whether the local CLI can run.

## Dependencies

None

## Scope

Add the health subcommand and focused unit coverage.

## Constraints

Use only the Go standard library and preserve existing command behavior.

## Success criteria

### C1: Tests pass

Check: go test ./...

### C2: Build succeeds

Check: go test ./...

## Verification

Run both checks from the repository root and report their exit codes.
```

## Invalid examples

Missing criteria (reject: `## Success criteria` must contain at least one
criterion/check pair):

```markdown
# TF-123: Empty task

## Goal

Do the work.

## Dependencies

None

## Scope

Implement it.

## Constraints

Keep it small.

## Success criteria

## Verification

Run checks.
```

Mismatched ID when saved as `TF-123-example.md` (reject: heading ID `TF-124`
does not match filename ID `TF-123`):

```markdown
# TF-124: Example

## Goal

Do the work.

## Dependencies

None

## Scope

Implement it.

## Constraints

Keep it small.

## Success criteria

### C1: It works

Check: go test ./...

## Verification

Run the check.
```

Malformed dependency (reject: dependency line must be a full `TF-NNN` ID):

```markdown
# TF-123: Example

## Goal

Do the work.

## Dependencies

- TF-12

## Scope

Implement it.

## Constraints

Keep it small.

## Success criteria

### C1: It works

Check: go test ./...

## Verification

Run the check.
```

## Migration from current task files

Existing task files use the same general Markdown headings but do not yet follow
this precise grammar. Before enabling v1 validation, migrate every task file in
`tasks/inbox/`, `tasks/ready/`, `tasks/active/`, `tasks/failed/`, and
`tasks/archive/`: normalize heading order and required fields, replace prose
dependency variants with `None` or ID list items, pair every success criterion
with a `Check:` line, and add Claim metadata to active and previously claimed
failed or archived tasks when available. Existing archived tasks without claim
data remain valid without a Claim block; migration must not invent an owner or
commit. New transitions from active to archive retain their Claim block as
specified above. The current TF-001 ready task lacks criterion/check pairs and
requires migration. This story does not rewrite task files or move them between
lifecycle directories.

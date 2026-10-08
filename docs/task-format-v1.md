# Task file format v1

This document defines state-specific on-disk contracts for task files. V1 uses
restricted Markdown so people can review and edit task contracts without a
separate authoring tool. The validator recognizes only the syntax below; it does
not interpret general Markdown, HTML, front matter, or formatting extensions.
This keeps task parsing in the standard library and adds no Go dependency. TOML
remains the format for project policy in `.taskfactory/config.toml`.

## File and identity rules

A task file name is `<ID>-<slug>.md`. An ID matches `TF-[0-9]{3}` and is unique
across all five task directories. A slug is one or more lowercase ASCII letters,
digits, or single hyphens; it cannot begin or end with a hyphen. The first line
must be `# <ID>: <title>`; its ID must exactly equal the filename ID and its
title must be non-empty single-line text. The slug is not derived from or
compared with the title. These identity checks apply in every state, including
inbox. Duplicate IDs or malformed filenames are errors even when the file is in
inbox.

The only valid state directories are `inbox`, `ready`, `active`, `failed`, and
`archive`; a task file under any other state directory is invalid. Empty
placeholder files such as `.gitkeep` are not task files. Task files use UTF-8,
LF line endings, and no tabs. Inbox bodies are unrestricted proposal text after
the required first heading. Complete executable contracts use the restricted
syntax below: headings and their order are exact; blank lines may separate
blocks; lists use `- ` with one item per line; text fields may wrap until the
next heading or list item. Inline HTML, nested lists, tables, links, and fenced
code blocks are not part of an executable contract. Verification commands are
single-line values and are not parsed as shell syntax by the task validator.

## State-specific contracts

The path under `tasks/` determines which contract applies. Validation never
changes a task's state.

### Inbox

Inbox contains proposals that may be incomplete. Validate only the common file,
identity, encoding, and uniqueness rules above. The body after the first heading
may use ordinary Markdown and may omit or add sections, dependencies, criteria,
and checks. In particular, a proposal need not yet have a Goal or Verification
section. Do not resolve dependencies or interpret its body as an executable
contract. Promotion to ready is the point at which the complete contract below
becomes mandatory; validation itself does not promote it.

### Ready

A ready task must satisfy the complete executable contract below and must not
contain a `## Claim` block.

### Active

An active task must satisfy the complete executable contract and include one
complete `## Claim` block as specified under Lifecycle metadata and evidence.

### Failed

A failed task must satisfy the complete executable contract. Its `## Claim`
block is optional: when present, it must be complete and valid and is retained
unchanged from the active task; absence is allowed for work that failed or was
blocked before a claim. Failure details belong in attempt evidence, not in
fabricated claim metadata.

### Archive

An archived task must satisfy the complete executable contract. Its Claim block
may be absent for legacy records made before claim metadata was introduced. If
present, it must be complete and valid and is retained unchanged. Do not infer
or fabricate missing claim or verification history.

Legacy archived records may also retain the historical success-criteria form:
one or more `- <text>` list items under `## Success criteria`, with no `Check:`
lines. An item may continue across following non-list lines until the next item
or section heading. This exception applies only in `tasks/archive/`; it
preserves the original record without inventing commands or implying that a
check was run. New archive entries must use the complete executable contract,
including criterion/check pairs. Archived records may retain at most one
optional `## Outcome` section with non-empty prose after `## Verification` or
after a retained `## Claim`; it records the historical result and is not
verification evidence. The legacy exceptions do not relax identity, required
contract headings, dependencies, or other field rules.

## Complete executable contract

A complete executable contract contains these headings exactly once and in this
order:

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
it is not rewritten to represent later events. An active-to-archive transition
requires the retained Claim block even though static validation permits legacy
archive files without one. Failed tasks may omit the block only when they were
never claimed. Missing metadata is not inferred or fabricated. If an optional
Claim block is present, it must be complete and valid.

Each worker verification attempt is recorded by appending one JSON object and LF
to `.taskfactory/evidence/<ID>.jsonl`. This file is append-only: existing bytes
must never be changed or removed. It is created on first attempt. Each line is
one UTF-8 JSON object with exactly these top-level keys and types:

- `task_id`: string; the task ID.
- `attempt`: positive integer; starts at 1 and increases by one for each
  successfully appended attempt for this task.
- `recorded_at`: string; RFC 3339 UTC timestamp.
- `outcome`: string; exactly `PASS`, `FAILED`, or `BLOCKED`.
- `branch`: string; claimed branch.
- `base_commit`: string; full lowercase hexadecimal Git object ID (40 or 64
  characters).
- `result_commit`: string; full lowercase hexadecimal Git object ID (40 or 64
  characters), or the empty string if no result commit exists.
- `changed_files`: array of strings; sorted, unique, repository-relative paths
  changed from `base_commit` in the worker worktree, including untracked files.
- `checks`: array of command-result objects, in execution order. Each object has
  exactly `source` (string: `task` or `worker`), `criterion` (string: the
  criterion ID such as `C1`, or empty for a configured worker command),
  `command` (string: exact configured command), `exit_code` (integer, or null if
  the shell could not be started), `output` (string: combined stdout and stderr,
  empty if none), and `error` (string: process-start error, or empty).
- `note`: required string; it may be empty and carries optional human context,
  but never replaces structured command results.

A verification runs every task criterion check in listed order, followed by
`verification.worker` commands in array order. Both sets use the command
execution rules in `docs/config-v1.md` and run at the claimed task worktree
root. The first non-zero exit code or shell start error stops the sequence;
checks that did not run are omitted. A non-zero exit code makes the outcome
`FAILED`. A shell start error records `exit_code: null` and its message in
`error`, stops the sequence, and makes the outcome `BLOCKED`. `PASS` means all
checks in both sets ran and exited zero. `changed_files` is a snapshot taken
when the attempt is recorded. An attempt with no changes uses an empty array.

The following are independent examples; each uses attempt 1 and is not part of a
claimed historical sequence. A passing attempt has this shape:

```json
{
  "task_id": "TF-123",
  "attempt": 1,
  "recorded_at": "2026-10-08T12:00:00Z",
  "outcome": "PASS",
  "branch": "feature/TF-123",
  "base_commit": "0123456789abcdef0123456789abcdef01234567",
  "result_commit": "1123456789abcdef0123456789abcdef01234567",
  "changed_files": ["cmd/app/main.go", "cmd/app/main_test.go"],
  "checks": [
    {
      "source": "task",
      "criterion": "C1",
      "command": "go test ./...",
      "exit_code": 0,
      "output": "ok\t./...",
      "error": ""
    },
    {
      "source": "worker",
      "criterion": "",
      "command": "go vet ./...",
      "exit_code": 0,
      "output": "",
      "error": ""
    }
  ],
  "note": ""
}
```

A failed attempt records the first non-zero result and omits commands that did
not run:

```json
{
  "task_id": "TF-123",
  "attempt": 1,
  "recorded_at": "2026-10-08T12:05:00Z",
  "outcome": "FAILED",
  "branch": "feature/TF-123",
  "base_commit": "0123456789abcdef0123456789abcdef01234567",
  "result_commit": "1123456789abcdef0123456789abcdef01234567",
  "changed_files": ["cmd/app/main.go"],
  "checks": [
    {
      "source": "task",
      "criterion": "C1",
      "command": "go test ./...",
      "exit_code": 1,
      "output": "FAIL\t./...",
      "error": ""
    }
  ],
  "note": ""
}
```

A blocked attempt records a shell start failure with a null exit code:

```json
{
  "task_id": "TF-123",
  "attempt": 1,
  "recorded_at": "2026-10-08T12:06:00Z",
  "outcome": "BLOCKED",
  "branch": "feature/TF-123",
  "base_commit": "0123456789abcdef0123456789abcdef01234567",
  "result_commit": "",
  "changed_files": [],
  "checks": [
    {
      "source": "task",
      "criterion": "C1",
      "command": "go test ./...",
      "exit_code": null,
      "output": "",
      "error": "start sh: executable not found"
    }
  ],
  "note": ""
}
```

To append safely, a verifier acquires an exclusive per-task lock before reading
evidence or choosing an attempt number. While holding it, the verifier validates
every existing line as a complete JSON object with exactly the schema keys and
types above. It rejects malformed JSON, non-object lines, unknown or missing
keys, wrong field types or values, and records whose `task_id` does not match
the task. It also requires positive attempt numbers starting at 1 with no
duplicates or gaps, in line order. For a missing or empty file, the next attempt
is 1; otherwise it is one greater than the final validated attempt. It appends
exactly one complete JSON object followed by LF and releases the lock after the
append. Concurrent verifiers for the same task therefore serialize and receive
distinct consecutive numbers. If any existing line is invalid or the file does
not end in LF, the verifier must report an error and leave all existing bytes
byte-for-byte unchanged; it must not repair, truncate, or reuse an attempt
number. Evidence from prior runs is never inferred or fabricated. A successful
worker report alone does not archive a task: integration must succeed before the
task moves to `tasks/archive/`. Evidence records worker attempts and do not
replace integration evidence required by the protocol.

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

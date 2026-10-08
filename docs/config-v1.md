# TaskFactory configuration contract v1

This document is the binding configuration contract for the future Go loader.
Configuration is project policy; task state stays in task files. It defines the
complete v1 schema, defaults, validation, command execution, and worktree path.
It does not add Go code, a module dependency, or a project config file.

## File and project root

The file is `<repository-root>/.taskfactory/config.toml`. The repository root
is the top-level directory reported by Git for the current working tree. A
command started in a subdirectory uses that same root; a command outside a Git
working tree has no project root and reports an error. There is no upward search
for a config in a parent repository or nested directory.

If the file is absent, the loader uses every default below. `init` writes a
complete config with those defaults. If the file exists but cannot be read or
parsed, loading fails before the requested operation changes project state.

## TOML parser

Use `github.com/BurntSushi/toml`, a focused, established Go TOML decoder, rather
than maintaining a partial TOML parser. TOML has string escaping, table rules,
and numeric forms that are easy to parse incorrectly; this library provides
those rules and metadata for detecting undecoded keys. Decode into typed
configuration structs, apply defaults explicitly, and reject every undecoded
key or table. This adds one runtime dependency with a narrow configuration
parsing purpose; it does not require adding any other framework. No dependency
version or `go.mod` change is part of this contract task.

## Schema and defaults

Keys and table names are case-sensitive. Values must have the stated TOML type;
there is no string-to-number or string-to-boolean coercion. Every key is
optional and takes the listed default when absent. An omitted table behaves as
if all its keys were omitted.

| TOML key | TOML type | Default | Validation |
| --- | --- | --- | --- |
| `protocol_version` | integer | `1` | Must equal `1`; any other integer is an unsupported protocol version. |
| `workers.max_parallel` | integer | `4` | Must be from `1` through `4`, inclusive. |
| `git.use_worktrees` | boolean | `true` | `true` creates an isolated task worktree; `false` uses the repository working tree directly. |
| `git.integration_strategy` | string | `"ff-only"` | Must equal `"ff-only"`; v1 supports no other integration strategy. |
| `integration.stop_on_main_failure` | boolean | `true` | When true, a failed main verification stops integration and the task is not archived. |
| `verification.worker` | array of strings | `["go test ./..."]` | Must contain at least one non-empty command string. |
| `verification.integration` | array of strings | `["go test ./...", "go vet ./..."]` | Must contain at least one non-empty command string. |
| `verification.main` | array of strings | omitted (disabled) | When present, must contain at least one non-empty command string; an empty array is invalid. |

The maximum worker count is a v1 invariant, not a tunable ceiling: values above
four are rejected. An absent `verification.main` means no post-merge main
verification commands are run. This keeps main verification optional while
preserving the technical-spec defaults for worker and integration verification.

## Validation and errors

Loading fails before a command changes task, Git, or worktree state when any of
the following applies:

- The file is unreadable, malformed TOML, or contains a value of the wrong TOML
  type.
- `protocol_version` is not `1`.
- `workers.max_parallel` is less than `1` or greater than `4`.
- `git.integration_strategy` is not `"ff-only"`.
- A configured verification array is empty or contains an empty string.
- Any key or table is not listed in the schema above.

Error text must identify the key or table and the observed problem. For
unsupported protocol versions, report the supplied version and that only v1 is
supported. For a worker limit, report the supplied value and allowed range
`1..4`. Unknown keys are errors, not ignored extensions; a later protocol
version can add keys deliberately.

## Verification command representation and execution

Each verification setting is a TOML array of strings. Each string is one full
command line, and array order is execution order. TaskFactory runs each command
separately through a POSIX-compatible `sh -c` found on `PATH`; shell operators,
quoting, expansions, and pipelines therefore have shell meaning. It does not
split a command string itself. Git operations are invoked directly without a
shell. Repository verification commands are trusted project policy.

The loader rejects empty strings but does not try to parse shell syntax. A
missing `sh` or a command that exits unsuccessfully is a verification failure.
The process inherits the caller's environment, runs synchronously, and exposes
combined stdout and stderr as evidence while preserving the command's exit
status. A non-zero command stops the current verification list; later commands
in that list are not run. Worker commands run at the claimed task worktree root.
Integration commands run at the integration candidate worktree root after
rebasing onto the current main branch and before advancing main. Optional main
commands run at the repository root after a successful fast-forward update of
main. If `stop_on_main_failure` is true and main verification fails, stop the
pipeline and do not archive the task.

The initial supported platforms are macOS and Linux. On another platform,
verification requires a POSIX-compatible `sh` available on `PATH`; failure to
start it is reported as a verification error. This shell requirement is
separate from Git invocation and keeps the sample's shell command strings
unambiguous.

## Worktree policy and paths

With `git.use_worktrees = true`, `claim` creates a worktree for task `TF-123`
at:

```text
<repository-root>/.taskfactory/worktrees/TF-123
```

The task ID is the validated stable ID from the task contract and is used as
one path component. The directory's parent is created as needed. Claim fails if
that task worktree path is already occupied by an unrelated directory or
worktree; it does not overwrite it. With `git.use_worktrees = false`, claim
assigns the task in the repository working tree and does not create this
directory. The worker limit applies in either mode.

## Complete valid example

This example sets every configurable key, including the optional main command
set, and is valid as written:

```toml
protocol_version = 1

[workers]
max_parallel = 4

[git]
use_worktrees = true
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = ["go test ./..."]
integration = ["go test ./...", "go vet ./..."]
main = ["go test ./..."]
```

## Invalid examples

Unsupported protocol version:

```toml
protocol_version = 2
```

Expected error: `protocol_version 2 is unsupported; only version 1 is
supported`.

Worker count above the v1 maximum:

```toml
[workers]
max_parallel = 5
```

Expected error: `workers.max_parallel 5 is outside the allowed range 1..4`.

Unknown table:

```toml
[experimental]
cache_ttl = 60
```

Expected error: `unknown table [experimental]`.

Wrong type for a schema key:

```toml
[git]
use_worktrees = "yes"
```

Expected error: `git.use_worktrees must be a boolean`.

## Technical-spec example coverage

The technical-spec example's keys all have exact types, defaults, and
validation above: `protocol_version`, `workers.max_parallel`,
`git.use_worktrees`, `git.integration_strategy`,
`integration.stop_on_main_failure`, `verification.worker`, and
`verification.integration`. The additional `verification.main` key is
explicitly optional and disabled when omitted.

# TaskFactory configuration contract v1

This document is the binding configuration contract for the future Go loader.
Configuration is project policy; task state stays in task files. It defines the
complete v1 schema, defaults, validation, command execution, and worktree path.
It does not add Go code, a module dependency, or a project config file.

## File and project root

The file is `<repository-root>/.taskfactory/config.toml`. The repository root is
the top-level directory reported by Git for the current working tree. A command
started in a subdirectory uses that same root; a command outside a Git working
tree has no project root and reports an error. There is no upward search for a
config in a parent repository or nested directory.

Every command that needs project state requires this file. If it is absent, the
command fails with an actionable error that identifies the expected path and
instructs the user to run `taskfactory init` from the repository. `init` is the
only state-changing command that may run before the file exists; it creates the
`.taskfactory` directory, task directories, and a complete config with the
defaults below. `--help` and `--version` do not require a project config. If the
file exists but cannot be read or parsed, loading fails before the requested
operation changes project state.

## TOML parser

Use `github.com/BurntSushi/toml`, a focused, established Go TOML decoder, rather
than maintaining a partial TOML parser. TOML has string escaping, table rules,
and numeric forms that are easy to parse incorrectly; this library provides
those rules and metadata for detecting undecoded keys. Decode into typed
configuration structs, apply defaults explicitly, and reject every undecoded key
or table. This adds one runtime dependency with a narrow configuration parsing
purpose; it does not require adding any other framework. No dependency version
or `go.mod` change is part of this contract task.

## Schema and defaults

Keys and table names are case-sensitive. Values must have the stated TOML type;
there is no string-to-number or string-to-boolean coercion. When the config file
exists, every key is optional and takes the listed default when absent. An
omitted table behaves as if all its keys were omitted. Absence of the config
file itself is an error for commands that need project state, as described
above.

| TOML key                           | TOML type        | Default                             | Validation                                                                                                                 |
| ---------------------------------- | ---------------- | ----------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `tasks_dir`                        | string           | `"tasks"`                           | Relative slash path inside the state directory; see "The tasks_dir key" below.                                             |
| `protocol_version`                 | integer          | `1`                                 | Must equal `1`; any other integer is an unsupported protocol version.                                                      |
| `workers.max_parallel`             | integer          | `4`                                 | Must be from `1` through `4`, inclusive.                                                                                   |
| `retry_budget`                     | integer          | `5`                                 | Must be at least `1`; the attempts a task may use before `requeue` refuses it. See "The retry_budget key" below.           |
| `git.use_worktrees`                | boolean          | `true`                              | Must be `true`; `false` is invalid because v1 requires isolated worker worktrees.                                          |
| `git.worktree_root`                | string           | `".taskfactory/worktrees"`          | Non-empty path. Relative paths resolve from repository root; absolute paths are allowed, including outside the repository. |
| `git.integration_strategy`         | string           | `"ff-only"`                         | Must equal `"ff-only"`; v1 supports no other integration strategy.                                                         |
| `git.base_branch`                  | string           | `"main"`                            | A valid local branch name; see "The git.base_branch key" below.                                                            |
| `integration.stop_on_main_failure` | boolean          | `true`                              | When true, a failed main verification stops integration and the task is not archived.                                      |
| `verification.worker`              | array of strings | `["go test ./..."]`                 | Must contain at least one non-empty command string.                                                                        |
| `verification.integration`         | array of strings | `["go test ./...", "go vet ./..."]` | Must contain at least one non-empty command string.                                                                        |
| `verification.main`                | array of strings | omitted (disabled)                  | When present, must contain at least one non-empty command string; an empty array is invalid.                               |

The maximum worker count is a v1 invariant, not a tunable ceiling: values above
four are rejected. `git.use_worktrees` is retained as an explicit policy key,
but v1 rejects `false`; all workers use isolated worktrees. An absent
`verification.main` means no post-merge main verification commands are run. This
keeps main verification optional while preserving the technical-spec defaults
for worker and integration verification.

## The retry_budget key

`retry_budget` is a top-level integer key. It is the number of attempts a task
may use before `taskfactory requeue` refuses to requeue it. The default is `5`,
a limit: a project without the key refuses a requeue once a task has used five
attempts. The value must be at least `1`; `0`, a negative number or a string is
rejected with an error naming `retry_budget`.

The budget is checked only by `requeue`, against the last attempt number in the
task's evidence file. A refusal names the task ID, the attempts used and the
budget, and changes no file and creates no commit. Workers and `work` do not
read the key. Requeue renames the evidence file aside when it resets the
counter, so the count is the attempts since the last requeue.

## The tasks_dir key

`tasks_dir` is a top-level string key, next to `protocol_version` and
`project_root`, not part of a table. It names the folder that holds the five
task state directories (`inbox`, `ready`, `active`, `failed`, `archive`), as a
slash path relative to the state root, which is the project unless
`--state-dir` or `TF_STATE_DIR` names another directory. The default is
`"tasks"`. A project without the key behaves exactly as before. Setting it to
`.taskfactory/tasks` keeps the task folders beside the config while the config,
evidence, locks, logs and worktrees stay in `.taskfactory/`.

The config file itself stays at `<state>/.taskfactory/config.toml`. The value is
cleaned (`./a//b/` becomes `a/b`). It is rejected when it is empty, absolute, `.`
or contains a `..` element or a backslash, when it resolves to the state root or
to `.taskfactory` itself, when it equals one of the runtime paths under
`.taskfactory` (`evidence`, `integration-evidence`, `logs`, `worktrees`, the lock
files, the stop marker, `config.toml`) or lies inside one, and when a symlink in
the path leaves the state root. `.taskfactory/tasks` is not ignored by the
`.gitignore` lines `init` writes, because those name only the runtime paths, so
the task folders stay trackable. `init` never writes the key; when the config
already sets it, `init` creates the state directories there.

## The git.base_branch key

`git.base_branch` names the local branch that `claim` creates task branches from
(`refs/heads/<name>`), that `integrate` rebases onto and fast-forwards, that
`check-main` verifies and that `requeue` checks a task branch against before it
removes it. The default is `"main"`, so a project without the key behaves
exactly as before. TaskFactory never detects the branch: a repository whose
default branch is `master` sets `base_branch = "master"`.

The value must be a plain branch name that Git accepts: it is not empty, has no
space, control character or any of `~ ^ : ? * [ \`, does not start with `-`, `/`
or `.`, does not end with `.` or `/`, contains no `..`, `@{` or `//`, and no
path component starts with `.` or ends with `.lock`. A bad value fails the load
with an error naming `git.base_branch`. Error messages that mention the base
branch name the configured branch, for example `local refs/heads/master is
unavailable`. Integration still requires that branch checked out in the
repository worktree.

## Validation and errors

Loading fails before a command changes task, Git, or worktree state when any of
the following applies:

- A command other than `init`, `--help`, or `--version` needs project state but
  `.taskfactory/config.toml` is absent. The error identifies the missing path
  and says to run `taskfactory init` from the repository.
- The file is unreadable, malformed TOML, or contains a value of the wrong TOML
  type.
- `protocol_version` is not `1`.
- `workers.max_parallel` is less than `1` or greater than `4`.
- `git.integration_strategy` is not `"ff-only"`.
- `git.use_worktrees` is `false`.
- `git.worktree_root` is not a non-empty string path, or cannot be resolved to a
  usable absolute directory path.
- `tasks_dir` breaks a rule in "The tasks_dir key". The error names the key and
  the config file: `tasks_dir must be a relative path inside the state
  directory`.
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

The loader rejects empty strings but does not try to parse shell syntax. For
worker verification, failure to start `sh` produces `BLOCKED`; a command that
starts and exits non-zero produces `FAILED`. The process inherits the caller's
environment, runs synchronously, and exposes combined stdout and stderr as
evidence while preserving the command's exit status. A non-zero command stops
the current verification list; later commands in that list are not run. For
`taskfactory verify <ID>`, run the task's criterion checks in their listed order
first, then run `verification.worker` in array order. These two lists form one
sequence: the first non-zero exit code or shell start error stops the sequence,
and later commands are not run. Record each command that ran, including combined
stdout and stderr and its exit status, in that order; do not add results for
commands that were skipped. Use the same `sh -c` execution rules for criterion
checks and configured worker commands. Worker commands run at the claimed task
worktree root. Integration commands run at the integration candidate worktree
root after rebasing onto the current main branch and before advancing main.
Optional main commands run at the repository root after a successful
fast-forward update of main. If `stop_on_main_failure` is true and main
verification fails, stop the pipeline and do not archive the task.

The initial supported platforms are macOS and Linux. On another platform,
verification requires a POSIX-compatible `sh` available on `PATH`; failure to
start it is reported as a verification error. This shell requirement is separate
from Git invocation and keeps the sample's shell command strings unambiguous.

## Worktree policy and paths

V1 requires `git.use_worktrees = true`. `claim` creates a separate Git worktree
for each task. `git.worktree_root` sets the directory that contains those
per-task worktrees. If the value is relative, resolve it against the repository
root, never the caller's current directory. If it is absolute, use that path; it
may be outside the repository. Normalize the resolved location to a canonical
absolute path before recording or comparing it. Create missing parent
directories as needed, and fail with an actionable error if the location cannot
be created or is not a directory.

For task `TF-123`, the path is:

```text
<resolved-git.worktree_root>/TF-123
```

The task ID is the validated stable ID from the task contract and is used as one
path component. The resulting path must be a descendant of the resolved worktree
root and must not contain `.` or `..` components. Claim fails if that task
worktree path is already occupied by an unrelated directory or worktree; it does
not overwrite it. The worker limit applies to these isolated worktrees.

Path resolution is lexical and filesystem-aware: resolve a relative value
against the repository root, clean it, make it absolute, and resolve existing
symlink components before using it. If the final directory does not exist,
resolve the nearest existing parent and append the remaining cleaned components.
This makes comparisons stable even when the configured location is outside the
repository or reached through a symlink.

## Complete valid example

This example sets every configurable key, including the optional main command
set, and is valid as written:

```toml
protocol_version = 1
tasks_dir = "tasks"

[workers]
max_parallel = 4

[git]
use_worktrees = true
worktree_root = ".taskfactory/worktrees"
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = ["go test ./..."]
integration = ["go test ./...", "go vet ./..."]
main = ["go test ./..."]
```

An installation that stores worktrees outside the repository may replace the
`worktree_root` value under `[git]` with an absolute root. For example, on macOS
or Linux:

```toml
worktree_root = "/var/tmp/taskfactory-worktrees"
```

For this setting, task `TF-123` uses `/var/tmp/taskfactory-worktrees/TF-123`.

## Invalid examples

Unsupported protocol version:

```toml
protocol_version = 2
```

Expected error:
`protocol_version 2 is unsupported; only version 1 is supported`.

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

A tasks_dir that leaves the state directory:

```toml
tasks_dir = "../elsewhere"
```

Expected error:
`tasks_dir must be a relative path inside the state directory`.

Disabled worker worktrees:

```toml
[git]
use_worktrees = false
```

Expected error:
`git.use_worktrees must be true in protocol v1; isolated worker worktrees are required`.

## Technical-spec example coverage

The technical-spec example's keys all have exact types, defaults, and validation
above: `protocol_version`, `workers.max_parallel`, `git.use_worktrees`,
`git.integration_strategy`, `git.worktree_root`,
`integration.stop_on_main_failure`, `verification.worker`, and
`verification.integration`. The additional `verification.main` key is explicitly
optional and disabled when omitted.

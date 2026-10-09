# TF-060: Make work usable in a project without worker instructions

## Goal

`taskfactory work` must not fail in a project made with `taskfactory init`
because `docs/worker-instructions.md` does not exist. When the file is
missing, the worker prompt carries a built-in default instruction text.

## Dependencies

- TF-055
- TF-056

## Scope

Found in the TF-057 end-to-end run: `work` refused after a successful claim
with "read docs/worker-instructions.md in worktree ...: no such file or
directory". The owner decided on a built-in fallback, not a file from `init`.

Behavior of `workprompt.Build`:

1. When the file exists in the claimed worktree, its bytes are embedded
   verbatim, exactly as today.
2. When it does not exist there, a short generic default text embedded in the
   package is used instead. The prompt says so with a marker line that starts
   with `Built-in worker instructions` and names the missing file.
3. Any other read failure, such as a directory or an unreadable file at that
   path, is still an error. Only "does not exist" falls back.
4. The default text covers: read the task contract, work only in the
   worktree, run the task checks and the project bin/test and bin/lint when
   they exist, commit with plain git commit, never change signing, report
   each command and exit code, stop and report blockers, do not weaken checks.

`init` does not write the file. Update the README quick start and
`docs/dispatch-design.md` to describe the fallback.

## Constraints

Do not change the rules block, task validation or any other command. No new
Go dependencies. The prompt stays deterministic and never reads the
environment or includes secrets. No Git signing override in product code.
Existing workprompt and work tests stay unchanged and green.

## Success criteria

### C1: Missing file falls back to the built-in text

Check: go test ./internal/workprompt -run TestBuildFallback

### C2: An existing file is still embedded verbatim

Check: go test ./internal/workprompt -run TestBuildVerbatim

### C3: A directory or unreadable file is still an error

Check: go test ./internal/workprompt -run TestBuildInstructionsReadError

## Verification

Run each check from the repository root, then `bin/test` and `bin/lint`.

# TF-053: Whole-tree validation skips archive contracts

## Goal

Make whole-tree validation (`taskfactory validate tasks`, `bin/test`, and the
`status` check) stop enforcing task contracts on archived files. Archived tasks
are history. The archive is still read so that task ID uniqueness and
dependency resolution run across the whole tree.

## Dependencies

- TF-003
- TF-045

## Scope

Change `internal/taskvalidate`, `cmd/taskfactory`, and the docs that describe
whole-tree validation. The rules are:

1. `validate tasks` and `Validate(root, "")` report contract diagnostics for
   inbox, ready, active, and failed files only. Archived files get no contract
   checks.
2. Every archived file is still read to find its task ID. A dependency is
   satisfied only when it names a file in `tasks/archive`. Archived files do
   not contribute dependency edges.
3. An archived file whose task ID cannot be read, or whose heading ID does not
   match its filename ID, or that is empty or not UTF-8, or that shares an ID
   with another task, is reported once as an archive read error at that file.
   This error is always reported, even in a single-file or inbox-only run.
4. `validate tasks/archive` and an explicit archived file path still run the
   full contract checks on exactly the files they name.
5. `validate` with no arguments still checks inbox and ready only.

## Constraints

Do not edit TF-040 or any other inbox task. Do not change the ready contract
or the exit codes. Do not add dependencies. Keep `verify`, `claim`, and
`status` output shapes unchanged apart from the documented validation scope.

## Success criteria

### C1: Whole-tree validation ignores broken archived contracts

Check: go test ./internal/taskvalidate -run 'TestTreeSkipsArchive' -v

### C2: Unreadable archived identities are reported once

Check: go test ./internal/taskvalidate -run 'TestTreeArchiveReadErrors' -v

### C3: Ready, dependency, and duplicate failures still fail the tree

Check: go test ./internal/taskvalidate -run 'TestTreeKeepsReadyFailures' -v

### C4: CLI passes whole tree with broken archive and fails the archive folder

Check: go test ./cmd/taskfactory -run 'TestTreeArchiveCLI' -v

### C5: Repository checks pass

Check: bin/test

## Verification

Run each success-criteria check with `-v` and confirm the named tests print
PASS. Then run `bin/test`, `bin/lint`, `go vet ./...`, `gofmt -l .`, and
`go run ./cmd/taskfactory validate tasks`. In a scratch copy of the repository,
break one archived task's contract and confirm `validate tasks` passes while
`validate tasks/archive` fails naming that file. Also break one archived
file's ID and confirm `validate tasks` reports it once as an archive read error.
Report each exit code.

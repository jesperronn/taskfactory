# TF-046: Share adapter code and cover the stall path

## Goal

Remove the duplicated launch plumbing in the three local worker adapters and
close the test gaps found when they were verified. The adapters keep their
harness-specific argv, preflight listings and notes, and their observable
behavior does not change.

## Dependencies

- TF-024
- TF-025
- TF-026

## Scope

The packages `internal/adapter/omp`, `internal/adapter/pi` and
`internal/adapter/claude` each define the same `State` and `Result` types,
`PreflightResult` and `PreflightCheck` with `Err`, a TCP dial helper, option
defaulting, and a run skeleton. That skeleton starts the process in the
worktree, captures output, maps exit to `exit`, timeout to `stalled`, and start
failure to `blocked`, and bounds the wait after a timeout. Move these shared
parts into `internal/adapter/common` and have each package use them through
type aliases or thin wrappers, so exported names used by tests stay. Keep the
harness-specific argv, preflight listing and notes in each package.

Add the missing tests with stubs only: a stall (timeout) test for `omp` and for
`claude`, matching the Pi package's `TestRunStallsAtAdapterTimeout`, and a Pi
preflight test fed with a fixture in the real `pi --list-models` format. The
fixture header is `provider  model  context  max-out  thinking  images`, with
rows such as `olla  Ornith-1.5-35B-A3B-MLX-4bit  128K  16.4K  no  no` and
`omlx  Ornith-1.5-35B-A3B-MLX-4bit  128K  16.4K  no  no`. The test asserts that
the omlx row matches and the olla-only row does not.

## Constraints

Do not change any adapter argv, flag, note text or exported behavior beyond
moving code. The adapters' permission and stall-to-evidence mappings are not
changed here. The three named tests of each adapter package must keep passing
unchanged, and their existing assertions must not be edited. Tests never start
a real model, open real sockets, or read the real home directory.

## Success criteria

### C1: The shared package has its own tests

Check: go test ./internal/adapter/common

### C2: All adapter packages pass after extraction

Check: go test ./internal/adapter/...

### C3: Stall tests pass for omp, pi and claude

Check: go test ./internal/adapter/... -run Stall

### C4: The Pi preflight accepts the real listing format

Check: go test ./internal/adapter/pi -run TestPreflightReadsListModelsFormat

### C5: The named TF-024, TF-025 and TF-026 tests still pass

Check: go test ./internal/adapter/... -run 'TestArgvSelectsExplicitModel|TestArgvUsesStdinAndNoFallback|TestPreflightRefusesBeforeLaunch|TestStubRunCreatesFileInTempWorktree'

### C6: Repository checks and task validation pass

Check: bin/test

## Verification

From the repository root, run `bin/test`, `bin/lint`, `go vet ./...`,
`gofmt -l .` and `go run ./cmd/taskfactory validate tasks`. Run each C1 to C5
check and report the exact command and exit code. Report the line count of each
adapter package before and after the extraction.

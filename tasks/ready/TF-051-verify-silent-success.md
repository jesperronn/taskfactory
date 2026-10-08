# TF-051: Print a summary line when verify runs

## Goal

`taskfactory verify <ID>` reports its outcome on stdout as one summary line, so
a worker does not need to read the evidence file to learn whether checks passed.

## Dependencies

None

## Scope

The verify command output and its help text in cmd/taskfactory, the verify
result in internal/verify, and tests for both passing and failing fixtures.

## Constraints

The evidence file format is unchanged and so are the exit codes: 0 for PASS,
1 for FAILED or BLOCKED, 2 for invalid usage. Color follows internal/ui rules,
so output is plain when piped. Do not edit .mdsmith.yml, bin/lint or the
bin/lint tests.

## Success criteria

### C1: A passing verify prints a PASS summary

Check: go test -count=1 -v -run TestVerifySummaryPass ./cmd/taskfactory | grep -q -- '--- PASS'

### C2: A failing verify prints a FAIL summary with the exit code

Check: go test -count=1 -v -run TestVerifySummaryFail ./cmd/taskfactory | grep -q -- '--- PASS'

### C3: Help describes the summary line

Check: go run ./cmd/taskfactory verify --help | grep -q 'summary line'

## Verification

Run each check from the repository root and report its exit code, then run
bin/test, bin/lint and go vet ./... and report their exit codes. Paste one pass
and one fail summary line from the CLI.

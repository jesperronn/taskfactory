# TF-058: Opt-in smoke test of work against the local oMLX server

## Goal

Add an opt-in test that drives `taskfactory work` against the real local oMLX
server and a real harness, and skips itself when the endpoint is down.

## Dependencies

- TF-056

## Scope

Add one Go test file `cmd/taskfactory/work_smoke_test.go` guarded by the
environment variable `TASKFACTORY_SMOKE=1` plus a TCP dial to `127.0.0.1:8000`.
When either is missing, the test calls `t.Skip` with the reason. The test
builds a temporary repository, claims a one-line task ("create smoke.txt with
one line"), runs `work` with an adapter and model named by
`TASKFACTORY_SMOKE_ADAPTER` and `TASKFACTORY_SMOKE_MODEL` (no defaults), and
checks exit 0, a log file under `.taskfactory/logs`, and that `verify` is the
printed next step. It never runs by default in `bin/test`.

## Constraints

No network use unless opted in. No default model. Set `commit.gpgsign` to
`false` in the temporary repository's own config only. Do not store the api
key. Keep the test to one adapter per run.

## Success criteria

### C1: Without the opt-in the test skips and exits 0

Check: go test ./cmd/taskfactory -run Smoke -v

### C2: The default test run is unchanged

Check: bin/test

## Verification

Run both checks, then once with `TASKFACTORY_SMOKE=1`, the adapter and model set
and the server up. Report exit codes and the skip line.

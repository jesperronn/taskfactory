# TF-106: Keep bin/test.test.sh cheap

## Goal

`bin/test.test.sh` can never re-run the real test suite, so it stays at a few
seconds, and a regression that makes it call a real `go` or the real
`bin/test` children fails loudly instead of adding minutes.

## Dependencies

None

## Scope

The first baseline measured `bin/test.test.sh` at 144s because it re-ran the
whole suite. After TF-093 its fixtures use stub `go` programs and stub shell
tests, and it takes about 3s with the slowest case near 0.3s. No task owns that
property, so it can regress silently. Fix that:

1. Give every fixture a `stubs/go` that records its call and, when the call is
not one a scenario expects, exits 99 with a message naming the call. A
fixture never reaches the real `go`: put the stubs first on `PATH` and also
set `GOFLAGS` to an invalid value so a real `go` fails at once.
2. Make fixtures copy only `bin/test` and `bin/testlib.sh`, never the real
`bin/*.test.sh` files or the repository tree.
3. Add one scenario, `isolation`, that runs `bin/test` in a bare fixture with
no stub and proves it exits without starting `go test`, and keep every
existing assertion unchanged.
4. Keep the real `bin/test --budget` scenarios that parse `go test -json`
on stub output; do not replace them with a real Go run.

## Constraints

Touch only `bin/test.test.sh`. Do not change `bin/test`, other tests or Go
code. Do not remove or weaken an existing assertion. Tests use temporary
directories only and leave the repository tree unchanged.

## Success criteria

### C1: The isolation scenario passes

Check: bin/test.test.sh | grep -q isolation

### C2: Every case stays under the shell case budget

Check: bin/test --budget bin/test.test.sh

### C3: The wrapper test finishes in under 10 seconds

Check: timeout 10 bin/test.test.sh

### C4: The whole task tree validates

Check: go run ./cmd/taskfactory validate .taskfactory/tasks

## Verification

Run `bin/test.test.sh`, each check, `bin/test` and `bin/lint` from the
repository root and report each exit code and the wrapper's total time.

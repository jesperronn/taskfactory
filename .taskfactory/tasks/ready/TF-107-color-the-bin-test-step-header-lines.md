# TF-107: Color the bin/test step header lines

## Goal

Every `==>` step header that `bin/test` prints uses the same bold cyan as the
shell test file headers, so the `==> go test ./...` and the task validation
headers no longer look different from the rest of the output.

## Dependencies

None

## Scope

- Today only the per-file shell test header (`bin/test` around line 124) is
  colored. The `==> go test` header (`run_go_tests`) and the
  `==> go run ./cmd/taskfactory validate` header are plain.
- Add one `step_header` function in `bin/test` that prints a header with the
  existing color rule: bold cyan when stdout is a terminal and `NO_COLOR` is
  unset, plain text otherwise. Use it for all three headers.
- Honour `FORCE_COLOR`: when it is set and non-empty, color even when stdout is
  not a terminal, unless `NO_COLOR` is set. This makes the color testable.
- Do not color the output of the tests themselves.
- Add a `color` scenario to `bin/test.test.sh` covering all three headers with
  and without color. Its `[PASS]` lines begin with `color:` as the checks expect.

## Constraints

Touch only `bin/test` and `bin/test.test.sh`. Header text and the exit codes
(0, 1, 2) stay as they are. Plain output must stay byte-identical when
`NO_COLOR` is set or stdout is not a terminal and `FORCE_COLOR` is unset. Tests
use stubs, never the real `go`, and never write in the real checkout.

## Success criteria

### C1: The go test header is colored when color is forced

Check: bin/test.test.sh | grep -q 'PASS.* color: go test header'

### C2: The validation header is colored when color is forced

Check: bin/test.test.sh | grep -q 'PASS.* color: validate header'

### C3: The shell test header stays colored when color is forced

Check: bin/test.test.sh | grep -q 'PASS.* color: shell test header'

### C4: Output is plain when NO_COLOR is set

Check: bin/test.test.sh | grep -q 'PASS.* color: plain when NO_COLOR'

### C5: The wrapper test and the whole suite still pass

Check: bin/test

## Verification

Run each check, `bin/test` and `bin/lint` from the repository root and report
each exit code. Report the three headers as printed with `FORCE_COLOR=1`.

# TF-113: Redact multi-line secrets in streamed worker output

## Goal

TF-109 streams worker output to the terminal and the log as it arrives and
redacts each complete line on its own. A credential value that contains a
newline, such as a PEM key, is split across lines, so no single line equals
the value and it reaches the terminal and the in-progress log unredacted. The
final log is rewritten from fully redacted output after the run, but the
terminal copy is not. Make streamed output redact multi-line credential
values too.

## Dependencies

- TF-109

## Scope

- In `internal/work/work.go`, change the `streamWriter` so a credential value
  spanning several lines is redacted before any of its lines is emitted, for
  example by holding back output while it could still be the start of a
  multi-line secret. Output must still arrive promptly when it cannot be.
- Add a test in `internal/work` that streams a multi-line secret through the
  writer in several chunks and shows no part of it reaches the terminal or
  log writers, while ordinary lines still arrive before the run exits.
- Keep the final log layout and the single-line redaction unchanged.

## Constraints

Only `internal/work` and its tests change. Redaction must never log a
credential value, not even in test failure messages. Do not touch commit
signing.

## Success criteria

### C1: A multi-line secret is not streamed

Check: go test ./internal/work -run TestStreamRedactsMultiLineSecret

### C2: Streaming still arrives before exit

Check: go test ./internal/work -run TestStreamsWorkerOutputBeforeExit

### C3: The full test suite passes

Check: bin/test

### C4: Lint passes

Check: bin/lint

## Verification

Run each check from the repository root and report each exit code.

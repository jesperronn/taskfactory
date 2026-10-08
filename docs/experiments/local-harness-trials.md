# Local harness trials with Ornith 35B

Trials of local coding harnesses against the oMLX server at 127.0.0.1:8000,
model `Ornith-1.5-35B-A3B-MLX-4bit`, October 2026. These are observations from
one machine, not a benchmark.

## Smoke task (2 minute limit)

Create `smoke.txt` containing one line, then `cat` it.

| Harness | Time | Result                               |
| ------- | ---- | ------------------------------------ |
| pi      | 7s   | exact content                        |
| omp     | 19s  | exact content                        |
| claude  | 12s  | content with a trailing period added |

## Small real task (10 minute limit)

Add `gofmt -l .` and `go vet ./...` to `bin/lint`, adjust the lint test, no
commit. Run one harness at a time.

| Harness | Time | Outcome                                                                            |
| ------- | ---- | ---------------------------------------------------------------------------------- |
| pi      | 22s  | works: lint test passes and a misformatted Go file makes `bin/lint` exit 1         |
| omp     | 600s | hit the limit; lint test passes but a misformatted Go file still passes `bin/lint` |

The omp change wraps `gofmt -l .` in an `if` on its exit status. gofmt exits 0
even when it lists unformatted files, so the check can never fail. The worker
reported nothing to verify this, which is why independent re-runs matter.

## Full lint task with three harnesses in parallel

Running the full TF-030 task on claude, pi and omp at once, all on one server,
produced no finished result in 30 minutes. Parallel runs share one model server
and slow each other; run local workers one at a time, or give each a small
slice.

## Practical notes

- Claude Code: pass the prompt on stdin when `--allowedTools` is used; the flag
  is variadic and swallows a positional prompt. Expect a harmless
  "model not in catalog" warning for Ornith names. The haiku alias in the
  environment selects the 9B model for its side calls.
- omp: `~/.omp/agent/config.yml` maps the `tiny` and `smol` roles to the 9B
  model, so side calls use it even when `--model` names the 35B. `--auto-approve`
  was refused by the permission classifier; omp print mode ran tools without it.
- pi: works against the `omlx` provider already present in its models file; no
  Olla proxy was needed for these runs.
- Worktrees need `npm ci` (now historical) or the pinned Go tool before
  `bin/test` can pass; check the exit code yourself.
- A worker's own report is not evidence; re-run `bin/test`, `bin/lint` and
  validation before merging.

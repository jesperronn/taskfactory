# TF-015 worker comparison

All three runs started from `a31153b7ebffbcf015741834b5b49dae9d308a24` in
separate checkouts and ran one at a time. Each was asked to implement
`tasks/ready/TF-015-lint-real-prettier-exit.md` without committing. Results
below distinguish worker output from independent checks.

| Harness              | Result                                                                                                                                                                                                             | Saved commit                                                                  |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------- |
| OMP 18.8.4           | No file changes or final response; stopped after several minutes. Initial sandbox launch failed on its home runtime directory; an allowed retry started but did not finish.                                        | `experiment/tf-015-omp` (run record only)                                     |
| Pi                   | Added a real-Prettier test, then attempted a repair. Both versions fail independent verification because the fixture copy of `bin/lint` changes to the parent of the temporary directory and scans other fixtures. | `acdcb6b` initial candidate; `96d4e3e` repair attempt; `experiment/tf-015-pi` |
| Claude Code via oMLX | Connected to `127.0.0.1:8000`; warned that the Ornith model was not in its local model catalog. No file changes or final result; stopped after several minutes.                                                    | `experiment/tf-015-claude` (run record only)                                  |

The Pi candidate's first independent real-Prettier check exited 1: its bad-file
assertion passed, but the clean-file assertion found other temporary `bad.md`
fixtures. The second candidate again failed the clean-file assertion and made
the mocked test fail by scanning repository Markdown. No `bin/lint` defect was
established.

The reviewed regression check in `bin/lint.real-prettier.test.sh` copies
`bin/lint` to a private `project/bin/` directory so its change to the project
root stays inside that fixture. With Prettier 3.9.9, the misformatted file was
named and `bin/lint` exited 1; the formatted file produced no warnings and
exited 0. The existing `bin/lint.test.sh` passed all four forwarding cases. The
reported exit-0 symptom did not reproduce with real Prettier;
`exec npx prettier` propagates Prettier's exit status.

# TF-051: Print a summary when verify passes

## Goal

`taskfactory verify <ID>` reports its outcome on stdout, so a worker does not
need to read the evidence file to learn whether checks passed.

## Scope

The verify command output only. The evidence format is unchanged.

## Notes

Found in docs/experiments/e2e-dry-run.md (D2). A passing verify printed
nothing and exited 0. worker-instructions.md tells workers to read the output.

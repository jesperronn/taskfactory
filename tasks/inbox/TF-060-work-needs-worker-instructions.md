# TF-060: Make work usable in a project without worker instructions

## Goal

`taskfactory work` must not fail in a project made with `taskfactory init`
because `docs/worker-instructions.md` does not exist.

## Scope

Found in the TF-057 end-to-end run: `work` refused with "read
docs/worker-instructions.md in worktree ...: no such file or directory" in a
fresh scratch project, after a successful claim. Either have `init` write a
default `docs/worker-instructions.md`, or have `work` fall back to a built-in
text when the file is missing. Mention the choice in the README quick start.

## Notes

Unrefined: the worktree is cut at claim time, so a file added to main later is
missing there until the branch is rebased.

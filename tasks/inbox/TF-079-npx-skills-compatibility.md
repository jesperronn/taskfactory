# TF-079: npx skills add compatibility and install docs

## Goal

Find out whether the `skills` CLI (`npx skills add jesperronn/taskfactory`)
discovers `skills/taskfactory/SKILL.md` in this repository layout, and
document the three ways to install the skill.

## Dependencies

- TF-072
- TF-078

## Scope

1. Research how the Vercel Labs `skills` CLI discovers skills, starting from
   <https://www.mintlify.com/vercel-labs/skills/introduction>. Record the
   discovery rules and whether `skills/taskfactory/SKILL.md` matches. If its
   list option can run against a local path without a global install (for
   example through `npx` only), run it and record the output; anything that
   cannot be run is written down as unverified, never as working.
2. Document in the README, under the Agent skill section, three install
   paths: `taskfactory skill install` (default `.agents/skills`, with
   `--target claude` and `--global`), `npx skills add` (only as far as step 1
   verified), and manual extraction of `taskfactory-skill_<version>.tar.gz`.
   State that nothing is bundled into the binary and that the checksum proves
   integrity, not publisher identity.
3. If the layout does not match, record the smallest change that would make
   it match as a new inbox task instead of changing the layout here.

## Constraints

Documentation only, plus the research record. No global installs and no
changes to the user's home directory. Do not edit archived tasks. Markdown
lines stay under 81 columns.

## Success criteria

### C1: The README documents the three install paths

Check: grep -q 'taskfactory skill install' README.md

### C2: The README mentions the npx skills command

Check: grep -q 'npx skills add' README.md

### C3: The README mentions manual extraction of the archive

Check: grep -q 'taskfactory-skill_' README.md

### C4: Docs lint clean

Check: bin/lint README.md

## Verification

Run every check from the repository root, then `bin/test` and `bin/lint`, and
report each exit code. In the report, separate what the `skills` CLI run
showed from what was only read in its docs. A human confirms
`npx skills add jesperronn/taskfactory` after the repository is public.

# TF-070: Release version parsing and bump advice

## Goal

Give `bin/release` its version handling and its conventional-commit bump
advice for stable versions, as isolated bash functions with shell tests.

## Dependencies

- TF-069

## Scope

Add `bin/release.lib.sh`, a library sourced by `bin/release` (executable not
required, no side effects when sourced). Functions:

1. A version parser accepting `v<major>.<minor>.<patch>` and rejecting
   everything else (no leading zeros, no prerelease label yet). It is one
   function, so a label can be added by the prerelease task.
2. A SemVer comparison function returning less, equal or greater.
3. A last-release-tag lookup: the highest tag matching the version regex by
   precedence, not by date. No tag prints nothing and returns non-zero.
4. A bump advice function that reads subjects and bodies from
   `git log <last-tag>..HEAD` (the full history when there is no tag), ignores
   merge, `fixup!`, `squash!` and `Revert` commits, and prints major, minor or
   patch by the monotone rule in `docs/release.md`: major for `!` or a
   breaking footer, minor for `feat`, patch for everything else. Subjects that
   do not match `type(scope)?!?: text` are printed as warnings. Docs-only
   history gives
   patch (nothing qualifies means patch).
5. A check function: refuse a version not greater than the last tag,
   warn (exit 0) when the version is lower than the advised bump, and for no
   tag require the human-given version without advice-based refusal.

Tests in `bin/release.version.test.sh` build temporary git repositories with
conventional-commit fixtures.

## Constraints

Bash only; no Go changes and no new dependencies. Tests never use the
network and set `commit.gpgsign` to false in the temporary repository's own
configuration only. The library never creates tags and never pushes.

## Success criteria

### C1: The parser accepts and rejects the right strings

Check: bin/release.version.test.sh parse

### C2: Precedence is numeric, not lexical

Check: bin/release.version.test.sh compare

### C3: The last tag is chosen by precedence

Check: bin/release.version.test.sh last-tag

### C4: Advice is monotone; docs-only gives patch, noise ignored

Check: bin/release.version.test.sh advice

### C5: The version check refuses and warns as specified

Check: bin/release.version.test.sh check

### C6: The whole test script passes

Check: bin/release.version.test.sh

## Verification

Run every check from the repository root, then `bin/test` and `bin/lint`, and
report each exit code. The test script accepts one optional group name and
runs everything when called with none.

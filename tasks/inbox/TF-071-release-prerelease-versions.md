# TF-071: Prerelease versions (-rc.N)

## Goal

Add `-rc.<N>` prerelease support to the version handling. This is the
separate prerelease subtask; it must land before the first release, which is
`v1.0.0-rc.1`.

## Dependencies

- TF-070

## Scope

Extend `bin/release.lib.sh` and its tests:

1. The parser accepts `v<major>.<minor>.<patch>` optionally followed by
   `-rc.<N>` (N a positive integer without leading zeros). Keep it one
   function with the label list in one place so more labels can be added
   later without a redesign.
2. SemVer precedence: `v1.0.0-rc.1` is lower than `v1.0.0-rc.2`, which is
   lower than `v1.0.0`. Prerelease numbers compare numerically.
3. Tag lookup helpers: the last stable tag, the last tag of any kind, and the
   next rc number for a base version from existing tags.
4. A predicate telling whether a version is a prerelease, used later to pass
   the GitHub prerelease flag.
5. Bump advice for a prerelease is computed from the last stable tag (or the
   full history when there is none) and the check function applies the same
   refuse-if-not-greater rule by precedence.

## Constraints

Bash only; no Go changes. Existing stable behaviour from TF-070 must not
change. No network and no tag creation outside temporary repositories whose
own config sets `commit.gpgsign` to false.

## Success criteria

### C1: Prerelease versions parse and bad labels are rejected

Check: bin/release.version.test.sh parse-rc

### C2: Precedence orders rc below the release

Check: bin/release.version.test.sh compare-rc

### C3: The next rc number comes from existing tags

Check: bin/release.version.test.sh next-rc

### C4: Stable results from TF-070 are unchanged

Check: bin/release.version.test.sh

## Verification

Run every check from the repository root, then `bin/test` and `bin/lint`, and
report each exit code.

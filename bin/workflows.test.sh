#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"

# Tests for bin/workflows. It only prints, so the tests run it as is.
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
workflows="${repo_root}/bin/workflows"

pass() { printf '[PASS] %s\n' "$1"; }
fail() { printf '[FAIL] %s\n' "$1" >&2; exit 1; }

assert_contains() { # assert_contains HAYSTACK NEEDLE CONTEXT
  [[ $1 == *"$2"* ]] || fail "$3: expected $2 in: $1"
  pass "$3"
}

status=0
out=$("${workflows}") || status=$?
(( status == 0 )) || fail "no argument exits 0"
pass "no argument exits 0"
for section in commands setup build test lint release; do
  assert_contains "${out}" "== ${section} ==" "prints the ${section} section"
done
assert_contains "${out}" "bin/test bin/lint.tells.test.sh" "documents running one test file"
assert_contains "${out}" "docs/release.md" "release points at the policy"

# Every executable script in bin/ (not a test) has a "# name — text" line, so
# the commands list never shows "(no description line)".
commands=$("${workflows}" commands)
[[ ${commands} != *"(no description line)"* ]] || fail "a script lacks its description line: ${commands}"
pass "every bin/ script has a description line"
for f in "${repo_root}"/bin/*; do
  [[ -f ${f} && -x ${f} && ${f} != *.test.sh ]] || continue
  [[ "$(head -c 2 "${f}")" == "#!" ]] || continue
  assert_contains "${commands}" "  ${f##*/} " "commands lists ${f##*/}"
done

status=0
"${workflows}" nope >/dev/null 2>&1 || status=$?
(( status == 2 )) || fail "unknown workflow exits 2, got ${status}"
pass "unknown workflow exits 2"

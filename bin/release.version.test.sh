#!/usr/bin/env bash
# Tests for bin/release.lib.sh. Usage: release.version.test.sh [section]
# Sections: parse compare last-tag advice check (default: all).
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null
export LC_ALL=C
# shellcheck source=bin/release.lib.sh
source "${repo_root}/bin/release.lib.sh"

work=$(mktemp -d)
trap 'rm -rf "${work}"' EXIT
assertions=0

pass() {
  assertions=$((assertions + 1))
  printf '[PASS] %s\n' "$1"
}
fail() {
  printf '[FAIL] %s\n' "$1" >&2
  exit 1
}
assert_eq() { # actual expected context
  [[ "$1" == "$2" ]] || fail "$3: expected '$2', got '$1'"
  pass "$3"
}
assert_contains() { # value needle context
  [[ "$1" == *"$2"* ]] || fail "$3: expected '$2' in '$1'"
  pass "$3"
}

repo_n=0
new_repo() {
  repo_n=$((repo_n + 1))
  local d="${work}/r${repo_n}"
  mkdir -p "${d}"
  git -C "${d}" init -q -b main
  git -C "${d}" config user.name tester
  git -C "${d}" config user.email tester@example.invalid
  git -C "${d}" config commit.gpgsign false
  git -C "${d}" config tag.gpgsign false
  cd "${d}"
}
commit() { # subject [body]
  if [[ $# -gt 1 ]]; then
    git commit -q --allow-empty -m "$1" -m "$2"
  else
    git commit -q --allow-empty -m "$1"
  fi
}
annotated() { git tag -a "$1" -m x; }

section_parse() {
  local v out
  for v in v0.0.0 v1.2.3 v10.20.30 v1.0.0; do
    out=$(release_parse_version "${v}") || fail "parse rejected ${v}"
    pass "accepts ${v} (${out})"
  done
  assert_eq "$(release_parse_version v1.22.333)" "1 22 333" "parse components"
  for v in 1.2.3 v1.2 v01.2.3 v1.02.3 v1.2.03 v1.2.3-rc.1 v1.2.3.4 v-1.2.3 "" latest "v1.2.3 " va.b.c; do
    if release_parse_version "${v}" >/dev/null; then fail "parse accepted '${v}'"; fi
    pass "rejects '${v}'"
  done
}

section_compare() {
  assert_eq "$(release_compare_versions v1.10.0 v1.9.0)" greater "v1.10.0 > v1.9.0"
  assert_eq "$(release_compare_versions v1.9.0 v1.10.0)" less "v1.9.0 < v1.10.0"
  assert_eq "$(release_compare_versions v1.2.3 v1.2.3)" equal "equal versions"
  assert_eq "$(release_compare_versions v2.0.0 v1.99.99)" greater "major dominates"
  assert_eq "$(release_compare_versions v1.2.10 v1.2.9)" greater "patch numeric"
  local rc=0
  release_compare_versions bogus v1.0.0 >/dev/null || rc=$?
  assert_eq "${rc}" 2 "invalid input returns 2"
}

section_last_tag() {
  new_repo
  commit "feat: a"
  local rc=0 out
  out=$(release_last_tag) || rc=$?
  assert_eq "${rc}" 1 "no tag returns non-zero"
  assert_eq "${out}" "" "no tag prints nothing"
  annotated v1.9.0
  commit "feat: b"
  annotated v1.10.0
  commit "fix: c"
  git tag v1.2.0 # lightweight, created last (newest by date), lower
  git tag -a v1.0.0-rc.1 -m x
  git tag latest
  assert_eq "$(release_last_tag)" v1.10.0 "highest by precedence, skips rc and latest"
  git tag v2.0.0
  assert_eq "$(release_last_tag)" v2.0.0 "lightweight tag handled by name"
  assert_eq "$(git tag --list | grep -c .)" 6 "library created no tags"
  new_repo
  commit "feat: a"
  git tag latest
  git tag v1.0.0-rc.1
  rc=0
  out=$(release_last_tag) || rc=$?
  assert_eq "${rc}" 1 "only non-matching tags returns non-zero"
}

advice_of() { release_bump_advice "$@" 2>/dev/null; }

section_advice() {
  local out err
  new_repo
  commit "docs: c"
  assert_eq "$(advice_of)" patch "no tag, docs only gives patch"
  commit "feat: a"
  assert_eq "$(advice_of)" minor "no tag uses full history"
  new_repo
  commit "feat: a"
  annotated v1.0.0
  commit "docs: c"
  commit "test: t"
  assert_eq "$(advice_of v1.0.0)" patch "docs-only range after tag is patch"
  commit "fix(core): b"
  assert_eq "$(advice_of v1.0.0)" patch "fix-only is patch"
  commit "feat: d"
  assert_eq "$(advice_of v1.0.0)" minor "feat range is minor"
  commit "feat(ui): scoped"
  commit "chore: f"
  assert_eq "$(advice_of v1.0.0)" minor "mixed: largest wins (minor over patch)"
  commit "feat!: bang"
  assert_eq "$(advice_of v1.0.0)" major "bang is major"
  new_repo
  commit "feat: a"
  annotated v1.0.0
  commit "fix(api)!: scoped bang"
  assert_eq "$(advice_of v1.0.0)" major "scoped bang is major"
  new_repo
  commit "feat: a"
  annotated v1.0.0
  commit "fix: x" "BREAKING CHANGE: e"
  assert_eq "$(advice_of v1.0.0)" major "BREAKING CHANGE footer is major"
  new_repo
  commit "feat: a"
  annotated v1.0.0
  commit "fix: x" "text

BREAKING-CHANGE: hyphen"
  assert_eq "$(advice_of v1.0.0)" major "BREAKING-CHANGE footer is major"
  new_repo
  commit "feat: a"
  annotated v1.0.0
  commit "fixup! feat: a"
  commit "squash! feat: a"
  commit "Revert \"feat: a\""
  commit "feat!: reverted noise" # kept: not noise
  assert_eq "$(advice_of v1.0.0)" major "real commit among noise still counts"
  new_repo
  commit "feat: a"
  annotated v1.0.0
  commit "fixup! feat!: a"
  commit "squash! feat!: a"
  commit "Revert \"feat!: a\""
  assert_eq "$(advice_of v1.0.0)" patch "fixup, squash and Revert ignored"
  git checkout -q -b side
  commit "feat!: on side"
  git checkout -q main
  git merge -q --no-ff --no-gpg-sign -m "Merge side" side
  git reset -q --hard HEAD~1 # drop merge; verify merges alone ignored below
  git merge -q --no-ff --no-gpg-sign -m "feat!: merge subject" side
  assert_eq "$(advice_of v1.0.0)" major "side commit still counted, merge commit not"
  new_repo
  commit "feat: a"
  annotated v1.0.0
  commit "random subject"
  commit "Feat: Upper"
  err=$(release_bump_advice v1.0.0 2>&1 >/dev/null)
  assert_contains "${err}" "random subject" "warning for non-conforming subject"
  assert_contains "${err}" "Feat: Upper" "warning for capitalised type"
  out=$(release_bump_advice v1.0.0 2>/dev/null)
  assert_eq "${out}" patch "non-conforming subjects count as no qualifying commit"
}

section_check() {
  local rc err
  new_repo
  commit "docs: a"
  rc=0
  release_check_version v1.0.0 >/dev/null 2>&1 || rc=$?
  assert_eq "${rc}" 0 "no tag: human version accepted"
  err=$(release_check_version v1.0.0 2>&1)
  assert_eq "${err}" "" "no tag: no advice warning"
  commit "feat!: big"
  rc=0
  release_check_version v0.1.0 >/dev/null 2>&1 || rc=$?
  assert_eq "${rc}" 0 "no tag: no advice-based refusal"
  rc=0
  release_check_version 1.0.0 >/dev/null 2>&1 || rc=$?
  assert_eq "${rc}" 1 "no tag: invalid version refused"
  rc=0
  release_check_version v1.0.0-rc.1 >/dev/null 2>&1 || rc=$?
  assert_eq "${rc}" 1 "no tag: rc refused until prerelease support"
  annotated v1.2.0
  commit "feat: n"
  for v in v1.2.0 v1.1.9 v0.9.0; do
    rc=0
    err=$(release_check_version "${v}" 2>&1) || rc=$?
    assert_eq "${rc}" 1 "refuses ${v} (not greater than v1.2.0)"
    assert_contains "${err}" "not greater" "refusal message for ${v}"
  done
  rc=0
  err=$(release_check_version v1.2.1 2>&1) || rc=$?
  assert_eq "${rc}" 0 "lower than advice still exits 0"
  assert_contains "${err}" "warning" "warns when lower than advised minor"
  rc=0
  err=$(release_check_version v1.3.0 2>&1) || rc=$?
  assert_eq "${rc}" 0 "matching advice exits 0"
  assert_eq "${err}" "" "matching advice is silent"
  commit "fix!: brk"
  err=$(release_check_version v1.3.0 2>&1) || true
  assert_contains "${err}" "advise major" "warns when lower than advised major"
  rc=0
  err=$(release_check_version v2.0.0 2>&1) || rc=$?
  assert_eq "${err}" "" "higher bump than advice is silent"
  assert_eq "${rc}" 0 "higher bump exits 0"
  new_repo
  commit "feat: a"
  annotated v1.9.0
  annotated_10() { git tag -a v1.10.0 -m x; }
  annotated_10
  rc=0
  release_check_version v1.9.5 >/dev/null 2>&1 || rc=$?
  assert_eq "${rc}" 1 "refused by numeric precedence against v1.10.0"
}

run_section() {
  assertions=0
  "section_${1//-/_}"
  [[ "${assertions}" -gt 0 ]] || fail "section $1 ran no assertions"
  printf '[OK] section %s: %s assertions\n' "$1" "${assertions}"
}

case "${1-all}" in
  parse | compare | last-tag | advice | check) run_section "$1" ;;
  all)
    for s in parse compare last-tag advice check; do run_section "${s}"; done
    ;;
  *)
    printf 'unknown section: %s (parse compare last-tag advice check)\n' "$1" >&2
    exit 2
    ;;
esac

#!/usr/bin/env bash
# Tests for bin/release-publish and .github/workflows/release.yml.
# Usage: bin/release-publish.test.sh [create|idempotent|refuse-lightweight|refuse-malformed|refuse-off-main|accept|failure|fallback|workflow]
# With no argument every section runs. Zero assertions is a failure.
# No network, no real gh, no token: a fake gh is placed first on PATH.
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"

export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/release-publish-test.XXXXXX")
case "${tmp}" in
  "${TMPDIR:-/tmp}"/*) ;;
  *) echo "unsafe temp dir: ${tmp}" >&2; exit 1 ;;
esac
trap 'if [[ -n "${tmp}" ]]; then rm -rf -- "${tmp}"; fi' EXIT

assertions=0
pass() { assertions=$((assertions + 1)); printf '[PASS] %s\n' "$1"; }
fail() { printf '[FAIL] %s\n' "$1" >&2; exit 1; }
assert_eq() { # actual expected context
  [[ "$1" == "$2" ]] || fail "$3: expected $(printf %q "$2"); got $(printf %q "$1")"
  pass "$3"
}

section=${1:-all}
case "${section}" in
  all|create|idempotent|refuse-lightweight|refuse-malformed|refuse-off-main|accept|failure|fallback|workflow) ;;
  *) echo "unknown section: ${section}" >&2; exit 2 ;;
esac
want() { [[ "${section}" == all || "${section}" == "$1" ]]; }

pub="${repo_root}/bin/release-publish"

# Fake gh. State dir FAKE_GH_DIR: calls.log (one line per call), exists
# (present = release exists), prerelease (true|false). FAKE_GH_CREATE_FAIL=1
# makes create fail; FAKE_GH_VIEW_FAIL=1 makes view fail for another reason.
mkdir -p "${tmp}/fakebin"
cat > "${tmp}/fakebin/gh" <<'FAKE'
#!/usr/bin/env bash
echo "$*" >> "${FAKE_GH_DIR}/calls.log"
[[ "$1" == release ]] || exit 3
case "$2" in
  view)
    if [[ -n "${FAKE_GH_VIEW_FAIL:-}" ]]; then echo "HTTP 500" >&2; exit 1; fi
    [[ -f "${FAKE_GH_DIR}/exists" ]] || { echo "release not found" >&2; exit 1; }
    if [[ "$*" == *"--json isPrerelease"* ]]; then cat "${FAKE_GH_DIR}/prerelease"; fi
    ;;
  create)
    [[ -z "${FAKE_GH_CREATE_FAIL:-}" ]] || { echo "boom" >&2; exit 1; }
    : > "${FAKE_GH_DIR}/exists"
    if [[ "$*" == *" --prerelease "* ]]; then echo true > "${FAKE_GH_DIR}/prerelease"; else echo false > "${FAKE_GH_DIR}/prerelease"; fi
    ;;
  edit) echo "${4#--prerelease=}" > "${FAKE_GH_DIR}/prerelease" ;;
  upload) ;;
  *) exit 3 ;;
esac
FAKE
chmod +x "${tmp}/fakebin/gh"
export PATH="${tmp}/fakebin:${PATH}"

n=0
new_gh_state() { # sets FAKE_GH_DIR; arg: exists-prerelease value or empty
  n=$((n + 1))
  FAKE_GH_DIR="${tmp}/gh${n}"
  export FAKE_GH_DIR
  mkdir -p "${FAKE_GH_DIR}"
  : > "${FAKE_GH_DIR}/calls.log"
  if [[ -n "${1-}" ]]; then : > "${FAKE_GH_DIR}/exists"; echo "$1" > "${FAKE_GH_DIR}/prerelease"; fi
}
calls() { cat "${FAKE_GH_DIR}/calls.log"; }

# make_repo <dir>: bare origin + work clone on main with one commit, pushed.
make_repo() {
  local d=$1
  mkdir -p "${d}"
  git init -q --bare -b main "${d}/origin.git"
  git init -q -b main "${d}/work"
  (
    cd "${d}/work"
    git config user.name Test
    git config user.email test@example.com
    git config commit.gpgsign false
    git config tag.gpgsign false
    git remote add origin "${d}/origin.git"
    echo a > a.txt
    git add a.txt
    git commit -q -m "feat: one"
    git push -q origin main
  )
}

# verify <repo-dir> <tag>: runs --verify-tag; sets rc and out.
verify() {
  rc=0
  out=$(cd "$1/work" && "${pub}" --verify-tag "$2" 2>&1) || rc=$?
}

dist="${tmp}/dist"
mkdir -p "${dist}"
echo a > "${dist}/a.tgz"
echo b > "${dist}/b.tgz"
echo s > "${dist}/sums.txt"
assets="${dist}/a.tgz ${dist}/b.tgz ${dist}/sums.txt"

if want create; then
  new_gh_state
  "${pub}" v1.0.0-rc.1 "${dist}" >/dev/null
  assert_eq "$(calls)" "release view v1.0.0-rc.1
release create v1.0.0-rc.1 --title v1.0.0-rc.1 --notes Release v1.0.0-rc.1 --prerelease ${assets}" "missing release is created with --prerelease for rc"
  new_gh_state
  "${pub}" v1.0.0 "${dist}" >/dev/null
  assert_eq "$(calls)" "release view v1.0.0
release create v1.0.0 --title v1.0.0 --notes Release v1.0.0 ${assets}" "missing stable release is created without --prerelease"
  rc=0
  "${pub}" 1.0.0 "${dist}" >/dev/null 2>&1 || rc=$?
  assert_eq "${rc}" 2 "invalid version to publish exits 2"
fi

if want idempotent; then
  new_gh_state false
  "${pub}" v1.0.0 "${dist}" >/dev/null
  assert_eq "$(calls)" "release view v1.0.0
release view v1.0.0 --json isPrerelease --jq .isPrerelease
release upload v1.0.0 ${assets} --clobber" "existing release: no create, upload with --clobber"
  new_gh_state true
  "${pub}" v1.0.0 "${dist}" >/dev/null
  assert_eq "$(calls | grep -c '^release edit v1.0.0 --prerelease=false$')" 1 "wrong prerelease flag on existing release is fixed"
  assert_eq "$(calls | grep -c '^release create')" 0 "no create when the release exists (flag fix)"
  # Second run after a create: the second run must not create again.
  new_gh_state
  "${pub}" v1.0.0-rc.1 "${dist}" >/dev/null
  "${pub}" v1.0.0-rc.1 "${dist}" >/dev/null
  assert_eq "$(calls | grep -c '^release create')" 1 "second run creates nothing again"
  assert_eq "$(calls | grep -c '^release upload v1.0.0-rc.1 .* --clobber$')" 1 "second run uploads with --clobber"
  assert_eq "$(calls | grep -c '^release edit')" 0 "second run leaves the matching prerelease flag alone"
fi

if want failure; then
  new_gh_state
  rc=0
  FAKE_GH_CREATE_FAIL=1 "${pub}" v1.0.0 "${dist}" >/dev/null 2>&1 || rc=$?
  [[ "${rc}" -ne 0 ]] || fail "gh create failure must propagate"
  pass "gh create failure exits non-zero (${rc})"
  new_gh_state
  rc=0
  FAKE_GH_VIEW_FAIL=1 "${pub}" v1.0.0 "${dist}" >/dev/null 2>&1 || rc=$?
  [[ "${rc}" -ne 0 ]] || fail "gh view failure must propagate"
  assert_eq "$(calls | grep -c '^release create')" 0 "an unreadable release state does not create"
fi

if want refuse-lightweight; then
  make_repo "${tmp}/lw"
  (cd "${tmp}/lw/work" && git tag v1.0.0 && git push -q origin v1.0.0)
  verify "${tmp}/lw" v1.0.0
  [[ "${rc}" -ne 0 ]] || fail "lightweight tag must be refused"
  case "${out}" in *"not annotated"*) pass "lightweight tag refused with message" ;; *) fail "message: ${out}" ;; esac
fi

if want refuse-malformed; then
  make_repo "${tmp}/mal"
  for t in v1.0 1.0.0 v1.0.0-beta.1 v01.0.0; do
    (cd "${tmp}/mal/work" && git tag -a "${t}" -m "Release ${t}")
    verify "${tmp}/mal" "${t}"
    [[ "${rc}" -ne 0 ]] || fail "malformed tag ${t} must be refused"
    case "${out}" in *"refusing ${t}"*) pass "malformed tag ${t} refused" ;; *) fail "message: ${out}" ;; esac
  done
fi

if want refuse-off-main; then
  make_repo "${tmp}/off"
  (
    cd "${tmp}/off/work"
    git checkout -q -b side
    echo s > s.txt
    git add s.txt
    git commit -q -m "feat: side"
    git push -q origin side
    git tag -a v1.0.0 -m "Release v1.0.0"
  )
  verify "${tmp}/off" v1.0.0
  [[ "${rc}" -ne 0 ]] || fail "tag off main must be refused"
  case "${out}" in *"not reachable from origin/main"*) pass "tag only on a side branch refused" ;; *) fail "message: ${out}" ;; esac
fi

if want accept; then
  make_repo "${tmp}/ok"
  for t in v1.0.0-rc.1 v1.0.0; do
    (cd "${tmp}/ok/work" && git tag -a "${t}" -m "Release ${t}")
    verify "${tmp}/ok" "${t}"
    assert_eq "${rc}" 0 "annotated ${t} on main accepted"
  done
fi

if want fallback; then
  make_repo "${tmp}/fb"
  (cd "${tmp}/fb/work" && git tag -a v1.0.0 -m "Release v1.0.0" && git update-ref -d refs/remotes/origin/main)
  verify "${tmp}/fb" v1.0.0
  assert_eq "${rc}" 0 "missing origin/main is fetched and the tag on main accepted"
  make_repo "${tmp}/fb2"
  (cd "${tmp}/fb2/work" && git tag -a v1.0.0 -m "Release v1.0.0" && git update-ref -d refs/remotes/origin/main && git remote set-url origin "${tmp}/does-not-exist.git")
  verify "${tmp}/fb2" v1.0.0
  [[ "${rc}" -eq 1 ]] || fail "unreachable origin must exit 1, got ${rc}"
  case "${out}" in *fetch*) pass "unreachable origin refused with a fetch message" ;; *) fail "message: ${out}" ;; esac
  make_repo "${tmp}/fb3"
  (
    cd "${tmp}/fb3/work"
    git checkout -q -b side
    echo s > s.txt
    git add s.txt
    git commit -q -m "feat: side"
    git tag -a v1.0.0 -m "Release v1.0.0"
    git update-ref -d refs/remotes/origin/main
  )
  verify "${tmp}/fb3" v1.0.0
  [[ "${rc}" -eq 1 ]] || fail "off-main tag must be refused after fallback fetch"
  case "${out}" in *"not reachable from origin/main"*) pass "off-main tag still refused after fallback fetch" ;; *) fail "message: ${out}" ;; esac
fi

if want workflow; then
  wf="${repo_root}/.github/workflows/release.yml"
  [[ -f "${wf}" ]] || fail "workflow file missing"
  need() { grep -Eq -- "$1" "${wf}" || fail "workflow lacks: $2"; pass "$2"; }
  need "^    tags:" "tags trigger"
  need "^      - 'v\\*'" "triggers on tags v*"
  assert_eq "$(grep -Ec '^on:|^  push:|^  pull_request|^  workflow_dispatch|^  schedule|^    branches' "${wf}")" 2 "only on: and push: triggers (no pull_request, branches, schedule)"
  need "^permissions:" "top-level permissions"
  need "^  contents: write$" "permissions contents: write"
  assert_eq "$(grep -c 'pull_request' "${wf}")" 0 "no pull_request anywhere"
  assert_eq "$(grep -o 'secrets\.[A-Za-z_]*' "${wf}" | sort -u | grep -vc '^secrets.GITHUB_TOKEN$' || true)" 0 "no secrets other than GITHUB_TOKEN"
  need "GH_TOKEN: \\$\\{\\{ github.token \\}\\}" "GH_TOKEN comes from github.token"
  need "go-version: '1.25.11'" "Go 1.25.11 pinned"
  need "fetch-depth: 0" "full history checkout"
  ci="${repo_root}/.github/workflows/ci.yml"
  assert_eq "$(grep -o 'actions/[a-z-]*@v[0-9]*' "${wf}")" "$(grep -o 'actions/[a-z-]*@v[0-9]*' "${ci}" | sed -n 1,2p)" "action majors match ci.yml"
  line_of() { grep -n -- "$1" "${wf}" | head -1 | cut -d: -f1; }
  v=$(line_of 'release-publish --verify-tag'); t=$(line_of 'run: bin/test'); l=$(line_of 'run: bin/lint')
  p=$(line_of 'run: bin/package'); u=$(line_of 'run: bin/release-publish "')
  [[ -n "${v}" && -n "${t}" && -n "${l}" && -n "${p}" && -n "${u}" ]] || fail "workflow steps not all found"
  [[ "${v}" -lt "${t}" && "${t}" -lt "${l}" && "${l}" -lt "${p}" && "${p}" -lt "${u}" ]] || fail "step order must be verify, test, lint, package, publish"
  pass "step order: verify-tag, test, lint, package, publish"
fi

if [[ "${assertions}" -eq 0 ]]; then
  echo "no assertions ran" >&2
  exit 1
fi
printf '%s assertions passed\n' "${assertions}"

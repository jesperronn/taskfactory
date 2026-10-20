#!/usr/bin/env bash
# Tests for bin/release.
# Usage: bin/release.test.sh [dry-run|gates|tag|signing|versions|no-signing-flags]
# With no argument every section runs. Zero assertions is a failure.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/release-test.XXXXXX")
case "${tmp}" in
  "${TMPDIR:-/tmp}"/*) ;;
  *) echo "unsafe temp dir: ${tmp}" >&2; exit 1 ;;
esac
trap 'if [[ -n "${tmp}" ]]; then rm -rf -- "${tmp}"; fi' EXIT

export GOTOOLCHAIN=${GOTOOLCHAIN:-local}
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null
release="${repo_root}/bin/release"
assertions=0

pass() { assertions=$((assertions + 1)); printf '[PASS] %s\n' "$1"; }
fail() { printf '[FAIL] %s\n' "$1" >&2; exit 1; }
assert_eq() { # actual expected context
  [[ "$1" == "$2" ]] || fail "$3: expected $(printf %q "$2"); got $(printf %q "$1")"
  pass "$3"
}
assert_has() { # text needle context
  case "$1" in *"$2"*) pass "$3" ;; *) fail "$3: missing '$2' in: $1" ;; esac
}
assert_lacks() { # text needle context
  case "$1" in *"$2"*) fail "$3: unexpected '$2' in: $1" ;; *) pass "$3" ;; esac
}

section=${1:-all}
case "${section}" in
  all|dry-run|gates|tag|signing|versions|no-signing-flags) ;;
  *) echo "unknown section: ${section}" >&2; exit 2 ;;
esac
want() { [[ "${section}" == all || "${section}" == "$1" ]]; }

# Stub package command: makes the same file names bin/package makes.
stub="${tmp}/stub-package"
cat > "${stub}" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
v=$1
mkdir -p "$2"
for n in "taskfactory_${v}_linux_amd64.tar.gz" "taskfactory-skill_${v}.tar.gz"; do : > "$2/${n}"; done
echo stub > "$2/SHA256SUMS"
STUB
chmod +x "${stub}"

# new_fixture <name>: sets fx (work repo) and origin (bare repo). main is
# pushed to origin and has one feat commit after the initial chore commit.
new_fixture() {
  local name=$1
  origin="${tmp}/${name}-origin.git"
  fx="${tmp}/${name}"
  git init -q --bare -b main "${origin}"
  git init -q -b main "${fx}"
  git -C "${fx}" config user.name Tester
  git -C "${fx}" config user.email tester@example.com
  git -C "${fx}" config commit.gpgsign false
  git -C "${fx}" remote add origin "${origin}"
  git -C "${fx}" commit -q --allow-empty -m "chore: start"
  git -C "${fx}" commit -q --allow-empty -m "fix: first fix"
  git -C "${fx}" push -q origin main
}
commit() { git -C "${fx}" commit -q --allow-empty -m "$1"; git -C "${fx}" push -q origin main; }
tag_old() { git -C "${fx}" tag -a "$1" -m "old $1"; }

# run_release <args...>: sets out (stdout+stderr) and status.
run_release() {
  status=0
  out=$(RELEASE_REPO_DIR="${fx}" RELEASE_TEST_CMD="${test_cmd:-true}" RELEASE_LINT_CMD="${lint_cmd:-true}" \
    RELEASE_PACKAGE_CMD="${package_cmd:-${stub}}" "${release}" "$@" 2>&1) || status=$?
}
snapshot() { # repo state: status, tags, HEAD, remote tags
  printf '%s\n%s\n%s\n%s\n' "$(git -C "${fx}" status --porcelain)" "$(git -C "${fx}" tag --list)" \
    "$(git -C "${fx}" rev-parse HEAD)" "$(git -C "${origin}" tag --list)"
}

if want dry-run; then
  new_fixture dry
  before=$(snapshot)
  run_release v1.0.0-rc.1
  assert_eq "${status}" 0 "dry run exits 0"
  assert_eq "$(snapshot)" "${before}" "dry run changes no status, tags or HEAD"
  assert_has "${out}" "git push origin v1.0.0-rc.1" "dry run prints the push command"
  assert_has "${out}" "SHA256SUMS" "dry run prints the checksums path"
  assert_has "${out}" "taskfactory-skill_v1.0.0-rc.1.tar.gz" "dry run prints artifact names"
  assert_has "${out}" "Dry run: nothing was changed" "dry run says nothing changed"
  assert_has "${out}" "last release tag: none" "first release has no last tag"
  assert_eq "$(git -C "${fx}" tag --list)" "" "dry run creates no tag"
  # One real, small package run (cross-build of the real project).
  status=0
  out=$(RELEASE_REPO_DIR="${fx}" RELEASE_TEST_CMD=true RELEASE_LINT_CMD=true "${release}" v1.0.0-rc.1 2>&1) || status=$?
  if [[ ${status} -eq 0 ]]; then
    assert_has "${out}" "taskfactory_v1.0.0-rc.1_linux_arm64.tar.gz" "real bin/package names the linux archive"
  else
    # The fixture has no Go module, so a real package run cannot build here.
    assert_has "${out}" "bin/package failed" "real bin/package is invoked and its failure is reported"
  fi
fi

if want gates; then
  new_fixture gates
  git -C "${fx}" checkout -q -b topic
  run_release v1.0.0-rc.1
  assert_eq "${status}" 1 "not on main exits 1"
  assert_has "${out}" "branch: on 'topic'" "not on main is named"
  git -C "${fx}" checkout -q main

  : > "${fx}/untracked.txt"
  run_release v1.0.0-rc.1
  assert_eq "${status}" 1 "untracked file exits 1"
  assert_has "${out}" "clean tree" "dirty tree is named"
  rm -f -- "${fx}/untracked.txt"

  git -C "${fx}" commit -q --allow-empty -m "feat: unpushed"
  run_release v1.0.0-rc.1
  assert_eq "${status}" 1 "ahead of origin exits 1"
  assert_has "${out}" "origin: local main is not equal" "origin divergence is named"
  git -C "${fx}" push -q origin main
  git -C "${fx}" reset -q --hard HEAD~1
  run_release v1.0.0-rc.1
  assert_has "${out}" "origin: local main is not equal" "behind origin is named"
  git -C "${fx}" pull -q --ff-only origin main

  tag_old v1.0.0-rc.1
  run_release v1.0.0-rc.1
  assert_eq "${status}" 1 "existing tag exits 1"
  assert_has "${out}" "tag: v1.0.0-rc.1 already exists" "existing tag is named"
  git -C "${fx}" tag -d v1.0.0-rc.1 >/dev/null

  test_cmd=false run_release v1.0.0-rc.1
  assert_eq "${status}" 1 "failing test command exits 1"
  assert_has "${out}" "tests: 'false' failed" "failing tests are named"
  lint_cmd=false run_release v1.0.0-rc.1
  assert_eq "${status}" 1 "failing lint command exits 1"
  assert_has "${out}" "lint: 'false' failed" "failing lint is named"

  # All failures are reported together.
  git -C "${fx}" checkout -q topic
  : > "${fx}/untracked.txt"
  test_cmd=false lint_cmd=false run_release v1.0.0-rc.1
  assert_eq "${status}" 1 "several failures exit 1"
  assert_has "${out}" "branch: on 'topic'" "combined: branch named"
  assert_has "${out}" "clean tree" "combined: dirty tree named"
  assert_has "${out}" "tests:" "combined: tests named"
  assert_has "${out}" "lint:" "combined: lint named"
  assert_has "${out}" "4 gate(s) failed" "combined: failure count"
  assert_eq "$(git -C "${fx}" tag --list)" "" "failed gates create no tag"
  rm -f -- "${fx}/untracked.txt"

  # No origin remote: a gate failure unless the test flag is set.
  new_fixture noorigin
  git -C "${fx}" remote remove origin
  run_release v1.0.0-rc.1
  assert_has "${out}" "origin: no origin remote" "missing origin fails by default"
  RELEASE_NO_ORIGIN_OK=1 run_release v1.0.0-rc.1
  assert_eq "${status}" 0 "RELEASE_NO_ORIGIN_OK=1 skips the origin gate"
  assert_has "${out}" "fetch and origin gate skipped" "skipping the fetch is announced"
fi

if want versions; then
  new_fixture ver
  run_release v1.0
  assert_eq "${status}" 2 "invalid version exits 2"
  run_release 1.0.0
  assert_eq "${status}" 2 "version without v exits 2"
  run_release v1.0.0-beta.1
  assert_eq "${status}" 2 "unknown label exits 2"
  run_release
  assert_eq "${status}" 2 "no version exits 2"
  run_release v1.0.0 --bogus
  assert_eq "${status}" 2 "unknown option exits 2"
  run_release v1.0.0-rc.1 v1.0.0-rc.2
  assert_eq "${status}" 2 "two versions exit 2"

  tag_old v1.0.0
  commit "feat: new thing"
  run_release v1.0.1
  assert_eq "${status}" 0 "lower than advice proceeds"
  assert_has "${out}" "warning: v1.0.1 is a patch bump but the commits advise minor" "lower than advice warns"
  assert_has "${out}" "advised bump (since v1.0.0): minor" "advice is printed"
  run_release v1.1.0
  assert_eq "${status}" 0 "matching the advice proceeds"
  assert_lacks "${out}" "warning:" "matching the advice does not warn"
  run_release v1.0.0
  assert_eq "${status}" 1 "equal to the last tag is refused"
  run_release v0.9.0
  assert_eq "${status}" 1 "lower than the last tag is refused"
  assert_has "${out}" "not greater than last tag v1.0.0" "refusal names the last tag"
  tag_old v1.1.0-rc.1
  run_release v1.1.0-rc.1
  assert_eq "${status}" 1 "an existing rc is refused"
  run_release v1.1.0-rc.2
  assert_eq "${status}" 0 "next rc proceeds"
fi

if want signing; then
  new_fixture sign
  before=$(snapshot)
  run_release v1.0.0-rc.1 --tag
  assert_eq "${status}" 1 "--tag without tag.gpgSign exits 1"
  assert_has "${out}" "signing: tag.gpgSign is not true" "refusal explains tag.gpgSign"
  assert_eq "$(snapshot)" "${before}" "refused tagging creates nothing"
  git -C "${fx}" config tag.gpgSign false
  run_release v1.0.0-rc.1 --tag
  assert_eq "${status}" 1 "--tag with tag.gpgSign false exits 1"
  assert_eq "$(git -C "${fx}" tag --list)" "" "tag.gpgSign false creates no tag"
  run_release v1.0.0-rc.1 --tag --allow-unsigned
  assert_eq "${status}" 0 "--allow-unsigned permits tagging"
  assert_eq "$(git -C "${fx}" cat-file -t v1.0.0-rc.1)" tag "--allow-unsigned creates an annotated tag"
  assert_eq "$(git -C "${origin}" tag --list)" "" "--allow-unsigned pushes nothing"
fi

if want tag; then
  new_fixture tag
  # tag.gpgSign true makes git itself sign, through gpg.program. The fixture
  # points it at a stand-in so no real key is needed; the script passes no flag.
  gpgstub="${tmp}/gpg-stub"
  cat > "${gpgstub}" <<'GPG'
#!/usr/bin/env bash
cat > /dev/null
echo "[GNUPG:] SIG_CREATED D 1 8 00 0 0 0 0" >&2
printf '%s\n' "-----BEGIN PGP SIGNATURE-----" stub "-----END PGP SIGNATURE-----"
GPG
  chmod +x "${gpgstub}"
  git -C "${fx}" config tag.gpgSign true
  git -C "${fx}" config gpg.program "${gpgstub}"
  run_release v1.0.0-rc.1 --tag
  assert_eq "${status}" 0 "--tag with tag.gpgSign true succeeds"
  assert_eq "$(git -C "${fx}" cat-file -t v1.0.0-rc.1)" tag "tag is annotated"
  assert_has "$(git -C "${fx}" cat-file -p v1.0.0-rc.1)" "Release v1.0.0-rc.1" "tag message is Release <version>"
  assert_eq "$(git -C "${fx}" tag -l --format='%(contents:subject)' v1.0.0-rc.1)" "Release v1.0.0-rc.1" "tag subject is exactly the message"
  assert_eq "$(git -C "${origin}" tag --list)" "" "nothing is pushed to origin"
  assert_has "${out}" "git push origin v1.0.0-rc.1" "the push command is printed for the human"
fi

if want no-signing-flags; then
  code=$(grep -v '^[[:space:]]*#' "${release}")
  for flag in ' -s ' ' -u ' '--sign' '--no-sign' '--local-user' ' -S'; do
    case "${code}" in
      *"git tag"*"${flag}"*) fail "bin/release passes forbidden flag '${flag}'" ;;
    esac
  done
  pass "git tag is called without signing flags"
  for word in 'git push' 'gh ' 'curl' 'wget' '--force-sign'; do
    case "${code}" in
      *"${word}"*) 
        # The printed hint 'git push origin' is an echo, not a call.
        if [[ "${word}" == 'git push' ]]; then
          if printf '%s\n' "${code}" | grep 'git push' | grep -qv '^[[:space:]]*echo '; then fail "bin/release runs git push"; fi
        else
          fail "bin/release contains '${word}'"
        fi ;;
    esac
  done
  pass "no push, gh or network tools are invoked"
  tagline=$(printf '%s\n' "${code}" | grep 'git tag')
  # shellcheck disable=SC2016
  assert_eq "${tagline}" '  git tag -a "${version}" -m "Release ${version}"' "the tag command is exactly git tag -a with the message"
fi

if (( assertions == 0 )); then
  echo "no assertions ran" >&2
  exit 1
fi
printf 'release tests: %d assertions passed\n' "${assertions}"

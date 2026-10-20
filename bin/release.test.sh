#!/usr/bin/env bash
# Tests for bin/release.
# Usage: bin/release.test.sh [dry-run|gates|tag|signing|versions|no-signing-flags|publish-needs-flag|publish-existing|publish-new|publish-race|publish-failures]
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
  all|dry-run|gates|tag|signing|versions|no-signing-flags|publish-needs-flag|publish-existing|publish-new|publish-race|publish-failures) ;;
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
  for word in 'curl' 'wget' '--force-sign'; do
    case "${code}" in
      *"${word}"*) fail "bin/release contains '${word}'" ;;
    esac
  done
  pass "no network tools or forced signing"
  # git push appears once, as a real call guarded by the --publish path
  # (the earlier exit 0 for a run without --publish); the hints are echoes.
  pushlines=$(printf '%s\n' "${code}" | grep 'git push' | grep -v '^[[:space:]]*echo ' || true)
  # shellcheck disable=SC2016
  assert_eq "${pushlines}" 'git push origin "${version}" || {' "git push is called exactly once with the tag"
  before_push=$(printf '%s\n' "${code}" | sed -n '1,/^git push origin/p')
  assert_has "${before_push}" "if (( ! do_publish )); then" "the push is preceded by the no --publish exit"
  # gh is only invoked inside gh_cmd.
  ghlines=$(printf '%s\n' "${code}" | grep -E '(^|[^_a-z])gh ' | grep -v 'gh_cmd' | grep -v '^[[:space:]]*\(echo\|printf\)' || true)
  assert_eq "${ghlines}" '  gh "$@"' "gh is only called through gh_cmd"
  tagline=$(printf '%s\n' "${code}" | grep 'git tag')
  # shellcheck disable=SC2016
  assert_eq "${tagline}" 'git tag -a "${version}" -m "Release ${version}"' "the tag command is exactly git tag -a with the message"
fi

# --- publish sections: fake gh first on PATH, bare local origin. ---
# Fake gh state dir FAKE_GH_DIR: calls.log (one line per call), exists
# (present = release exists), prerelease, views (counter).
# FAKE_GH_VIEW_FAIL=1: view fails with HTTP 500. FAKE_GH_CREATE_FAIL=1: create
# fails. FAKE_GH_APPEAR_AT=N: from the Nth view call on, the release exists.
mkdir -p "${tmp}/fakebin"
cat > "${tmp}/fakebin/gh" <<'FAKE'
#!/usr/bin/env bash
echo "$*" >> "${FAKE_GH_DIR}/calls.log"
[[ "$1" == release ]] || exit 3
case "$2" in
  view)
    c=$(cat "${FAKE_GH_DIR}/views" 2>/dev/null || echo 0)
    c=$((c + 1)); echo "${c}" > "${FAKE_GH_DIR}/views"
    if [[ -n "${FAKE_GH_VIEW_FAIL:-}" ]]; then echo "HTTP 500" >&2; exit 1; fi
    if [[ -n "${FAKE_GH_APPEAR_AT:-}" && ${c} -ge ${FAKE_GH_APPEAR_AT} && ! -f "${FAKE_GH_DIR}/exists" ]]; then
      : > "${FAKE_GH_DIR}/exists"; echo "${FAKE_GH_PRERELEASE:-true}" > "${FAKE_GH_DIR}/prerelease"
    fi
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

gh_n=0
new_gh() { # [existing-prerelease-value]
  gh_n=$((gh_n + 1))
  FAKE_GH_DIR="${tmp}/gh${gh_n}"
  export FAKE_GH_DIR
  mkdir -p "${FAKE_GH_DIR}"
  : > "${FAKE_GH_DIR}/calls.log"
  if [[ -n "${1-}" ]]; then : > "${FAKE_GH_DIR}/exists"; echo "$1" > "${FAKE_GH_DIR}/prerelease"; fi
}
gh_calls() { cat "${FAKE_GH_DIR}/calls.log"; }
gh_count() { grep -c "$1" "${FAKE_GH_DIR}/calls.log" || true; }
sign_fixture() { # signed tagging via a stand-in gpg.program
  local g="${tmp}/gpg-pub-stub"
  cat > "${g}" <<'GPG'
#!/usr/bin/env bash
cat > /dev/null
echo "[GNUPG:] SIG_CREATED D 1 8 00 0 0 0 0" >&2
printf '%s\n' "-----BEGIN PGP SIGNATURE-----" stub "-----END PGP SIGNATURE-----"
GPG
  chmod +x "${g}"
  git -C "${fx}" config tag.gpgSign true
  git -C "${fx}" config gpg.program "${g}"
}

if want publish-needs-flag; then
  new_fixture pnf; new_gh
  before=$(snapshot)
  run_release v1.0.0-rc.1 --publish
  assert_eq "${status}" 2 "--publish without --tag exits 2"
  assert_has "${out}" "--publish requires --tag" "usage error names the rule"
  assert_eq "$(snapshot)" "${before}" "usage error changes nothing"
  assert_eq "$(gh_calls)" "" "usage error never calls gh"
  run_release v1.0.0-rc.1
  assert_eq "${status}" 0 "dry run still exits 0"
  assert_eq "$(gh_calls)" "" "dry run never calls gh"
  assert_eq "$(snapshot)" "${before}" "dry run changes nothing"
  sign_fixture
  run_release v1.0.0-rc.1 --tag
  assert_eq "${status}" 0 "--tag without --publish succeeds"
  assert_eq "$(git -C "${origin}" tag --list)" "" "--tag without --publish pushes nothing"
  assert_eq "$(gh_calls)" "" "--tag without --publish never calls gh"
  assert_has "${out}" "Nothing was pushed" "--tag says nothing was pushed"
  run_release v1.0.0-rc.2 --tag --allow-unsigned
  assert_eq "$(git -C "${origin}" tag --list)" "" "--allow-unsigned without --publish pushes nothing"
fi

if want publish-existing; then
  new_fixture pex; new_gh true; sign_fixture
  before=$(snapshot)
  run_release v1.0.0-rc.1 --tag --publish
  assert_eq "${status}" 1 "an existing release exits 1"
  assert_has "${out}" "already exists" "existing release is named"
  assert_eq "$(git -C "${origin}" tag --list)" "" "existing release: origin has no tag"
  assert_eq "$(snapshot)" "${before}" "existing release: local repository unchanged (no local tag)"
  assert_eq "$(gh_count 'release create')" 0 "existing release: no create"
  assert_eq "$(gh_count 'release upload')" 0 "existing release: no upload"
fi

if want publish-failures; then
  new_fixture pfail; new_gh; sign_fixture
  before=$(snapshot)
  FAKE_GH_VIEW_FAIL=1 run_release v1.0.0-rc.1 --tag --publish
  assert_eq "${status}" 1 "gh view HTTP 500 exits 1"
  assert_has "${out}" "cannot read the release state" "unreadable state is named"
  assert_eq "$(snapshot)" "${before}" "unreadable state: nothing tagged or pushed"
  # Signing gate still applies to --publish.
  new_fixture pfail2; new_gh
  before=$(snapshot)
  run_release v1.0.0-rc.1 --tag --publish
  assert_eq "${status}" 1 "--publish without tag.gpgSign exits 1"
  assert_has "${out}" "signing: tag.gpgSign is not true" "signing gate applies to --publish"
  assert_eq "$(gh_calls)" "" "failed gate never calls gh"
  assert_eq "$(snapshot)" "${before}" "failed gate changes nothing"
  # Create fails after the push.
  new_fixture pfail3; new_gh; sign_fixture
  FAKE_GH_CREATE_FAIL=1 run_release v1.0.0-rc.1 --tag --publish
  assert_eq "${status}" 1 "create failure exits non-zero"
  assert_has "${out}" "WAS pushed" "create failure says the tag was pushed"
  assert_has "${out}" "workflow may have published" "create failure says the workflow may have published"
  assert_eq "$(git -C "${origin}" tag --list)" "v1.0.0-rc.1" "the tag is on origin after the create failure"
  # View failing only after the push.
  new_fixture pfail4; new_gh; sign_fixture
  # Fail the second view only: wrap via a flag file read by a dedicated fake.
  mkdir -p "${tmp}/fakebin2"
  cat > "${tmp}/fakebin2/gh" <<'FAKE2'
#!/usr/bin/env bash
# Second and later views fail with HTTP 500; the first says not found.
[[ "$2" == view ]] || exit 3
n=$(cat "${FAKE_GH_DIR}/n" 2>/dev/null || echo 0)
n=$((n + 1)); echo "${n}" > "${FAKE_GH_DIR}/n"
if [[ ${n} -ge 2 ]]; then echo "HTTP 500" >&2; exit 1; fi
echo "release not found" >&2; exit 1
FAKE2
  chmod +x "${tmp}/fakebin2/gh"
  PATH="${tmp}/fakebin2:${PATH}" run_release v1.0.0-rc.1 --tag --publish
  assert_eq "${status}" 1 "second view failing exits 1"
  assert_has "${out}" "WAS pushed" "second view failure says the tag was pushed"
fi

if want publish-new; then
  new_fixture pnew; new_gh; sign_fixture
  run_release v1.0.0-rc.1 --tag --publish
  assert_eq "${status}" 0 "publish of a new release exits 0"
  assert_has "${out}" "created by local publish" "the create path is named"
  assert_has "${out}" "check the Actions run" "the human is told to check the Actions run"
  assert_eq "$(git -C "${origin}" cat-file -t v1.0.0-rc.1)" tag "origin has an annotated tag"
  assert_eq "$(git -C "${origin}" tag -l --format='%(contents:subject)' v1.0.0-rc.1)" "Release v1.0.0-rc.1" "origin tag message"
  assert_eq "$(gh_count 'release view')" 3 "release state checked twice by bin/release plus once inside bin/release-publish"
  assert_eq "$(gh_count 'release create')" 1 "exactly one create call"
  assert_eq "$(gh_count 'release upload')" 0 "no upload on the create path"
  create=$(grep 'release create' "${FAKE_GH_DIR}/calls.log")
  assert_has "${create}" "release create v1.0.0-rc.1 --title v1.0.0-rc.1 --notes Release v1.0.0-rc.1 --prerelease " "rc create arguments"
  assert_has "${create}" "taskfactory-skill_v1.0.0-rc.1.tar.gz" "create includes the skill archive"
  assert_has "${create}" "SHA256SUMS" "create includes the checksums"
  # Stable: no --prerelease.
  new_fixture pnew2; new_gh; sign_fixture
  run_release v1.0.0 --tag --publish
  assert_eq "${status}" 0 "stable publish exits 0"
  create=$(grep 'release create' "${FAKE_GH_DIR}/calls.log")
  assert_has "${create}" "release create v1.0.0 --title v1.0.0 --notes Release v1.0.0 " "stable create arguments"
  assert_lacks "${create}" "--prerelease" "stable release is not a prerelease"
  # Unsigned path with --allow-unsigned.
  new_fixture pnew3; new_gh
  run_release v1.0.0-rc.1 --tag --publish --allow-unsigned
  assert_eq "${status}" 0 "--allow-unsigned publish exits 0"
  assert_eq "$(git -C "${origin}" tag --list)" "v1.0.0-rc.1" "unsigned publish pushed the tag"
fi

if want publish-race; then
  new_fixture prace; new_gh; sign_fixture
  FAKE_GH_APPEAR_AT=2 FAKE_GH_PRERELEASE=true run_release v1.0.0-rc.1 --tag --publish
  assert_eq "${status}" 0 "race path exits 0"
  assert_has "${out}" "release already created by the workflow, assets reconciled" "the reconcile path is named"
  assert_eq "$(git -C "${origin}" tag --list)" "v1.0.0-rc.1" "race: tag pushed once"
  assert_eq "$(gh_count 'release create')" 0 "race: no create call"
  assert_eq "$(gh_count 'release edit')" 0 "race: matching prerelease flag is not edited"
  assert_eq "$(gh_count 'release upload')" 1 "race: one upload call"
  up=$(grep 'release upload' "${FAKE_GH_DIR}/calls.log")
  assert_has "${up}" "release upload v1.0.0-rc.1 " "upload targets the version"
  assert_has "${up}" "--clobber" "upload uses --clobber"
  assert_has "${up}" "SHA256SUMS" "upload includes the checksums"
fi

if (( assertions == 0 )); then
  echo "no assertions ran" >&2
  exit 1
fi
printf 'release tests: %d assertions passed\n' "${assertions}"

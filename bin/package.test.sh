#!/usr/bin/env bash
# Tests for bin/package. Usage: bin/package.test.sh [archives|version|skill|checksums|refuse]
# With no argument every section runs. Zero assertions is a failure.
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/package-test.XXXXXX")
case "${tmp}" in
  "${TMPDIR:-/tmp}"/*) ;;
  *) echo "unsafe temp dir: ${tmp}" >&2; exit 1 ;;
esac
trap 'if [[ -n "${tmp}" ]]; then rm -rf -- "${tmp}"; fi' EXIT

export GOTOOLCHAIN=${GOTOOLCHAIN:-local}
export COPYFILE_DISABLE=1
v=v1.2.3-rc.4
assertions=0

pass() { assertions=$((assertions + 1)); printf '[PASS] %s\n' "$1"; }
fail() { printf '[FAIL] %s\n' "$1" >&2; exit 1; }

assert_eq() { # actual expected context
  [[ "$1" == "$2" ]] || fail "$3: expected $(printf %q "$2"); got $(printf %q "$1")"
  pass "$3"
}

section=${1:-all}
case "${section}" in
  all|archives|version|skill|checksums|refuse) ;;
  *) echo "unknown section: ${section}" >&2; exit 2 ;;
esac
want() { [[ "${section}" == all || "${section}" == "$1" ]]; }

if command -v sha256sum >/dev/null 2>&1; then
  sums_check=(sha256sum -c)
else
  sums_check=(shasum -a 256 -c)
fi

# A stand-in go is first on PATH, so the four cross builds are instant and write
# a fake binary that reports the version bin/package passed. Archive layout,
# checksums, version checks and refusals all run for real. The one real go build
# is the "host binary prints the version" case: it sets TESTLIB_REAL_GO=1 and
# builds the whole package once for real.
stub_bin="${tmp}/stub-bin"
mkdir -p "${stub_bin}"
testlib_install_go_stub "${stub_bin}"
export PATH="${stub_bin}:${PATH}"

# Build once and reuse for the sections that inspect archives.
out="${tmp}/out"
"${repo_root}/bin/package" "${v}" "${out}" >"${tmp}/package.log" 2>&1 || { cat "${tmp}/package.log" >&2; fail "bin/package succeeds"; }
pass "bin/package succeeds"

if want archives; then
  expected=$(printf '%s\n' SHA256SUMS \
    "taskfactory-skill_${v}.tar.gz" \
    "taskfactory_${v}_darwin_amd64.tar.gz" \
    "taskfactory_${v}_darwin_arm64.tar.gz" \
    "taskfactory_${v}_linux_amd64.tar.gz" \
    "taskfactory_${v}_linux_arm64.tar.gz")
  assert_eq "$(cd "${out}" && find . -mindepth 1 -maxdepth 1 | sed 's|^\./||' | LC_ALL=C sort)" "${expected}" "exactly the five archives and SHA256SUMS exist"
  for t in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
    assert_eq "$(tar -tzf "${out}/taskfactory_${v}_${t}.tar.gz" | LC_ALL=C sort)" "LICENSE
taskfactory" "${t} archive holds only the binary and LICENSE"
  done
fi

if want version; then
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) fail "unsupported host arch $(uname -m)" ;;
  esac
  # The one real build, for the host target only (TF_PACKAGE_TARGETS).
  # -trimpath is what bin/package passes already, so Go reuses compiled
  # packages; -buildvcs=false skips Go's git calls. One link is paid.
  real_out="${tmp}/real-out"
  TF_PACKAGE_TARGETS="${os}/${arch}" TESTLIB_REAL_GO=1 GOFLAGS=-buildvcs=false "${repo_root}/bin/package" "${v}" "${real_out}" >"${tmp}/real-package.log" 2>&1 || { cat "${tmp}/real-package.log" >&2; fail "real bin/package succeeds"; }
  pass "real bin/package build succeeds"
  mkdir "${tmp}/host"
  tar -xzf "${real_out}/taskfactory_${v}_${os}_${arch}.tar.gz" -C "${tmp}/host"
  assert_eq "$("${tmp}/host/taskfactory" --version)" "taskfactory version ${v}" "host binary prints the version"
fi

if want skill; then
  assert_eq "$(tar -tzf "${out}/taskfactory-skill_${v}.tar.gz" | LC_ALL=C sort)" "taskfactory/
taskfactory/SKILL.md" "skill archive is one taskfactory/ dir with only SKILL.md"
  if tar -tzf "${out}/taskfactory-skill_${v}.tar.gz" | grep -q '\.claude'; then fail "skill archive has no .claude paths"; fi
  pass "skill archive has no .claude paths"
  mkdir -p "${tmp}/badskill"
  printf -- '---\nname: broken\n---\n# no description\n' > "${tmp}/badskill/SKILL.md"
  status=0
  TF_PACKAGE_SKILL_DIR="${tmp}/badskill" "${repo_root}/bin/package" "${v}" "${tmp}/badskill-out" >/dev/null 2>&1 || status=$?
  assert_eq "${status}" 1 "broken SKILL.md frontmatter is rejected"
  if [[ -e "${tmp}/badskill-out" ]]; then fail "broken skill creates no output dir"; fi
  pass "broken skill creates no output dir"
fi

if want checksums; then
  (cd "${out}" && "${sums_check[@]}" SHA256SUMS >/dev/null 2>&1) || fail "SHA256SUMS verifies"
  pass "SHA256SUMS verifies"
  assert_eq "$(wc -l < "${out}/SHA256SUMS" | tr -d ' ')" 5 "SHA256SUMS covers five archives"
  mkdir "${tmp}/tamper"
  cp "${out}"/* "${tmp}/tamper/"
  printf 'x' >> "${tmp}/tamper/taskfactory-skill_${v}.tar.gz"
  status=0
  (cd "${tmp}/tamper" && "${sums_check[@]}" SHA256SUMS >/dev/null 2>&1) || status=$?
  [[ "${status}" -ne 0 ]] || fail "SHA256SUMS fails after tampering"
  pass "SHA256SUMS fails after tampering"
fi

if want refuse; then
  for bad in 1.2.3 v1.2 v01.2.3 v1.2.3-rc.0 v1.2.3-beta.1 v1.2.3-rc.1x ""; do
    status=0
    "${repo_root}/bin/package" "${bad}" "${tmp}/never" >/dev/null 2>&1 || status=$?
    assert_eq "${status}" 2 "bad version '${bad}' exits 2"
  done
  status=0
  "${repo_root}/bin/package" v1.0.0 >/dev/null 2>&1 || status=$?
  assert_eq "${status}" 2 "missing out-dir exits 2"
  for bad in v1.0.0-beta.1 v01.0.0 v1.00.0; do
    status=0
    "${repo_root}/bin/package" "${bad}" "${tmp}/never" >/dev/null 2>&1 || status=$?
    assert_eq "${status}" 2 "unknown label or leading zero '${bad}' exits 2"
  done
  # The acceptance runs check the version grammar and the output directory, not
  # the target list, so they build one target each (TF_PACKAGE_TARGETS).
  mkdir "${tmp}/ok-rc" "${tmp}/ok-stable"
  TF_PACKAGE_TARGETS=linux/amd64 "${repo_root}/bin/package" v1.0.0-rc.1 "${tmp}/ok-rc" >/dev/null 2>&1 || fail "v1.0.0-rc.1 accepted"
  pass "v1.0.0-rc.1 accepted"
  TF_PACKAGE_TARGETS=linux/amd64 "${repo_root}/bin/package" v1.0.0 "${tmp}/ok-stable" >/dev/null 2>&1 || fail "v1.0.0 accepted"
  pass "v1.0.0 accepted"
  src_out=$(bash -c 'before=$(set +o); source "$1"; after=$(set +o); [[ "${before}" == "${after}" ]] || echo OPTIONS-CHANGED' _ "${repo_root}/bin/release.lib.sh" 2>&1)
  assert_eq "${src_out}" "" "sourcing release.lib.sh prints nothing and changes no shell options"
  if [[ -e "${tmp}/never" ]]; then fail "bad version creates nothing"; fi
  status=0
  "${repo_root}/bin/package" "${v}" "${out}" >/dev/null 2>&1 || status=$?
  assert_eq "${status}" 1 "non-empty out-dir is refused"
  mkdir "${tmp}/empty"
  TF_PACKAGE_TARGETS=linux/amd64 "${repo_root}/bin/package" v1.0.0 "${tmp}/empty" >/dev/null 2>&1 || fail "empty out-dir is accepted"
  pass "empty out-dir is accepted"
fi

(( assertions > 0 )) || fail "no assertions ran"
printf 'OK: %s assertions\n' "${assertions}"

#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
scenarios=(tagged untagged dirty notag nogit output)

is_scenario() {
  local name=$1 s
  for s in "${scenarios[@]}"; do
    [[ "${s}" == "${name}" ]] && return 0
  done
  return 1
}

if (( $# == 0 )); then
  set -- "${scenarios[@]}"
fi
for name in "$@"; do
  if ! is_scenario "${name}"; then
    printf 'unknown scenario %q (known: %s)\n' "${name}" "${scenarios[*]}" >&2
    exit 2
  fi
done

fixture_root=$(mktemp -d)
trap 'rm -rf "${fixture_root}"' EXIT

# Most scenarios put a stand-in go first on PATH: it records the version bin/build
# passes and writes a small fake binary. The tagged scenario sets
# TESTLIB_REAL_GO=1 for one real go build, so the real compiler is still
# exercised end to end.
stub_bin="${fixture_root}/stub-bin"
mkdir -p "${stub_bin}"
testlib_install_go_stub "${stub_bin}"
export PATH="${stub_bin}:${PATH}"

assert_contains() {
  local value=$1
  local expected=$2
  local context=$3
  if [[ "${value}" != *"${expected}"* ]]; then
    printf '[FAIL] %s: expected to find %q in %q\n' "${context}" "${expected}" "${value}" >&2
    exit 1
  fi
  printf '[PASS] %s\n' "${context}"
}

assert_status() {
  local actual=$1
  local expected=$2
  local context=$3
  if [[ "${actual}" != "${expected}" ]]; then
    printf '[FAIL] %s: expected status=%s; actual status=%s\n' "${context}" "${expected}" "${actual}" >&2
    exit 1
  fi
  printf '[PASS] %s\n' "${context}"
}

# new_fixture copies bin/build and the module files into a fresh directory. A
# second argument, real, also copies the Go sources for a real go build.
new_fixture() {
  local name=$1
  mkdir -p "${fixture_root}/${name}/bin"
  cp "${repo_root}/bin/build" "${fixture_root}/${name}/bin/build"
  cp "${repo_root}/go.mod" "${repo_root}/go.sum" "${fixture_root}/${name}/"
  if [[ "${2:-}" == real ]]; then
    cp -R "${repo_root}/cmd" "${repo_root}/internal" "${repo_root}/skills" "${fixture_root}/${name}/"
  fi
}

# git_fixture creates a fixture that is its own Git repository with one commit.
# The repository is built once as a template and copied, so a scenario pays for
# one cp instead of git init, add and commit. Go sources for a real build are
# copied in as untracked files, which do not change git describe --dirty.
git_fixture() {
  local name=$1
  local template="${fixture_root}/template"
  if [[ ! -d "${template}" ]]; then
    new_fixture template
    git -C "${template}" init --quiet
    # One write instead of four git config calls.
    printf '[commit]\n\tgpgsign = false\n[tag]\n\tgpgsign = false\n[user]\n\tname = test\n\temail = test@example.invalid\n' \
      >>"${template}/.git/config"
    git -C "${template}" add -A
    git -C "${template}" commit --quiet -m fixture
  fi
  cp -R "${template}" "${fixture_root}/${name}"
  if [[ "${2:-}" == real ]]; then
    cp -R "${repo_root}/cmd" "${repo_root}/internal" "${repo_root}/skills" "${fixture_root}/${name}/"
  fi
}

# check_version builds in the fixture and checks the printed and embedded version.
check_version() {
  local name=$1
  local expected=$2
  local dir="${fixture_root}/${name}"
  local build_output version_output
  build_output=$(cd / && "${dir}/bin/build" 2>&1)
  assert_contains "${build_output}" "built dist/taskfactory (version ${expected})" "${name} PASS build prints version ${expected}"
  version_output=$("${dir}/dist/taskfactory" --version)
  assert_contains "${version_output}" "taskfactory version ${expected}" "${name} PASS binary reports ${expected}"
}

# The one real go build: a real binary must report the embedded version.
scenario_tagged() {
  git_fixture tagged real
  git -C "${fixture_root}/tagged" tag -a -m release v1.2.0
  printf '[PASS] tagged PASS fixture with real sources is ready\n'
  TESTLIB_REAL_GO=1 GOFLAGS="-trimpath -buildvcs=false" check_version tagged v1.2.0
}

scenario_untagged() {
  git_fixture untagged
  local dir="${fixture_root}/untagged"
  git -C "${dir}" tag -a -m release v1.2.0
  echo change >"${dir}/CHANGE.txt"
  git -C "${dir}" add CHANGE.txt
  git -C "${dir}" commit --quiet -m change
  local hash
  hash=$(git -C "${dir}" rev-parse --short HEAD)
  check_version untagged "v1.2.0-1-g${hash}"

  local custom_output usage_status=0 usage_output
  custom_output=$(cd / && "${dir}/bin/build" custom/tf-bin 2>&1)
  assert_contains "${custom_output}" 'built custom/tf-bin' 'untagged PASS accepts an optional output path'
  test -x "${dir}/custom/tf-bin"
  usage_output=$(cd / && "${dir}/bin/build" one two 2>&1) || usage_status=$?
  assert_status "${usage_status}" 2 'untagged PASS rejects more than one output path'
  assert_contains "${usage_output}" 'Usage: bin/build [output-path]' 'untagged PASS prints usage for extra arguments'
}

scenario_dirty() {
  git_fixture dirty
  local dir="${fixture_root}/dirty"
  git -C "${dir}" tag -a -m release v1.2.0
  echo "// dirty" >>"${dir}/go.mod"
  check_version dirty v1.2.0-dirty
}

scenario_notag() {
  git_fixture notag
  local hash
  hash=$(git -C "${fixture_root}/notag" rev-parse --short HEAD)
  check_version notag "dev-${hash}"
}

scenario_nogit() {
  new_fixture nogit
  check_version nogit dev
}

scenario_output() {
  git_fixture output
  local dir="${fixture_root}/output" out
  mkdir -p "${dir}/dist/sub"
  echo stale >"${dir}/dist/stale.tar.gz"
  echo stale >"${dir}/dist/sub/old"
  out=$(cd / && "${dir}/bin/build" 2>&1)
  assert_contains "${out}" 'built dist/taskfactory' 'output PASS default path is dist/taskfactory'
  test -x "${dir}/dist/taskfactory"
  test ! -e "${dir}/dist/stale.tar.gz" && test ! -e "${dir}/dist/sub"
  printf '[PASS] output PASS default build removes stale files in dist/\n'

  echo keep >"${dir}/dist/keep"
  out=$(cd / && "${dir}/bin/build" dist/custom-bin 2>&1)
  assert_contains "${out}" 'built dist/custom-bin' 'output PASS explicit path inside dist/ builds'
  test ! -e "${dir}/dist/keep" && test -x "${dir}/dist/custom-bin" && test ! -e "${dir}/dist/taskfactory"
  printf '[PASS] output PASS explicit path inside dist/ also empties dist/\n'

  echo keep >"${dir}/dist/keep"
  out=$(cd / && "${dir}/bin/build" other/tf-bin 2>&1)
  assert_contains "${out}" 'built other/tf-bin' 'output PASS explicit path outside dist/ is honored'
  test -x "${dir}/other/tf-bin" && test -e "${dir}/dist/keep"
  printf '[PASS] output PASS explicit path outside dist/ leaves dist/ alone\n'

  # Guards: a path that only looks like dist/ must not trigger removal.
  out=$(cd / && "${dir}/bin/build" dist/../other/tf-bin2 2>&1)
  test -x "${dir}/other/tf-bin2" && test -e "${dir}/dist/keep"
  out=$(cd / && "${dir}/bin/build" /dev/null/x 2>&1) && { echo '[FAIL] impossible path built' >&2; exit 1; }
  test -e "${dir}/dist/keep"
  mkdir -p "${dir}/distfoo" && echo keep >"${dir}/distfoo/keep"
  (cd / && "${dir}/bin/build" distfoo/tf-bin >/dev/null 2>&1)
  test -e "${dir}/distfoo/keep" && test -e "${dir}/dist/keep"
  printf '[PASS] output PASS dist/.. and dist-prefixed paths never remove anything\n'

  # A symlinked dist/ resolves outside the repo, so nothing is removed.
  local target="${fixture_root}/symtarget" status=0
  mkdir -p "${target}" && echo keep >"${target}/keep"
  rm -rf "${dir}/dist" && ln -s "${target}" "${dir}/dist"
  out=$(cd / && "${dir}/bin/build" 2>&1) || status=$?
  test -e "${target}/keep"
  printf '[PASS] output PASS symlinked dist/ target is left intact (status %s)\n' "${status}"

  grep -qx 'dist/' "${repo_root}/.gitignore"
  if grep -q 'bin/out' "${repo_root}/.gitignore"; then
    echo '[FAIL] .gitignore still lists bin/out' >&2
    exit 1
  fi
  printf '[PASS] output PASS .gitignore lists dist/ and not bin/out/\n'
}

for name in "$@"; do
  "scenario_${name}"
done

#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture_root=$(mktemp -d)
trap 'rm -rf "${fixture_root}"' EXIT

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

# new_fixture copies only the Go module and bin/build into a fresh directory.
new_fixture() {
  local name=$1
  mkdir -p "${fixture_root}/${name}/bin"
  cp "${repo_root}/bin/build" "${fixture_root}/${name}/bin/build"
  cp "${repo_root}/go.mod" "${repo_root}/go.sum" "${fixture_root}/${name}/"
  cp -R "${repo_root}/cmd" "${repo_root}/internal" "${fixture_root}/${name}/"
}

new_fixture git-build
git -C "${fixture_root}/git-build" init --quiet
git -C "${fixture_root}/git-build" add -A
git -C "${fixture_root}/git-build" -c user.name=test -c user.email=test@example.invalid commit --quiet -m fixture
expected_version=$(git -C "${fixture_root}/git-build" describe --always --dirty)

build_output=$(cd / && "${fixture_root}/git-build/bin/build" 2>&1)
assert_contains "${build_output}" 'built bin/out/taskfactory' 'default output path is bin/out/taskfactory'
test -x "${fixture_root}/git-build/bin/out/taskfactory"
printf '[PASS] bin/build creates an executable binary\n'

version_output=$("${fixture_root}/git-build/bin/out/taskfactory" --version)
assert_contains "${version_output}" "taskfactory version ${expected_version}" 'binary prints the git describe version'

custom_output=$(cd / && "${fixture_root}/git-build/bin/build" custom/tf-bin 2>&1)
assert_contains "${custom_output}" 'built custom/tf-bin' 'accepts an optional output path'
test -x "${fixture_root}/git-build/custom/tf-bin"
printf '[PASS] optional output path is honored\n'

usage_status=0
usage_output=$(cd / && "${fixture_root}/git-build/bin/build" one two 2>&1) || usage_status=$?
assert_status "${usage_status}" 2 'rejects more than one output path'
assert_contains "${usage_output}" 'Usage: bin/build [output-path]' 'prints usage for extra arguments'

new_fixture plain-build
build_output=$(cd / && "${fixture_root}/plain-build/bin/build" 2>&1)
version_output=$("${fixture_root}/plain-build/bin/out/taskfactory" --version)
assert_contains "${version_output}" 'taskfactory version dev' 'falls back to dev outside a Git checkout'

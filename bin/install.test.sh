#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture_root=$(mktemp -d)
# Go's module cache is read-only, so make the tree writable before removal.
cleanup() {
  chmod -R u+w "${fixture_root}" 2>/dev/null || true
  rm -rf "${fixture_root}"
}
trap cleanup EXIT

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

# The fixture is a small Git checkout holding only the Go module and the
# install and build scripts, so destinations never touch the real repository.
fixture="${fixture_root}/install"
mkdir -p "${fixture}/bin"
cp "${repo_root}/bin/install" "${repo_root}/bin/build" "${fixture}/bin/"
cp "${repo_root}/go.mod" "${repo_root}/go.sum" "${fixture}/"
cp -R "${repo_root}/cmd" "${repo_root}/internal" "${fixture}/"
git -C "${fixture}" init --quiet
git -C "${fixture}" add -A
git -C "${fixture}" -c user.name=test -c user.email=test@example.invalid commit --quiet -m fixture
expected_version=$(git -C "${fixture}" describe --always --dirty)
install_script="${fixture}/bin/install"

# run_install runs bin/install from / with PREFIX, GOBIN and GOPATH cleared,
# then applies the given NAME=value assignments.
run_install() {
  (cd / && env -u PREFIX -u GOBIN -u GOPATH "$@" "${install_script}" 2>&1)
}

dest_root="${fixture_root}/dest"

prefix_dir="${dest_root}/prefix/nested"
output=$(run_install PREFIX="${prefix_dir}")
assert_contains "${output}" "installed ${prefix_dir}/taskfactory" 'PREFIX: prints the installed path'
test -x "${prefix_dir}/taskfactory"
printf '[PASS] PREFIX: installs an executable binary\n'
assert_contains "$("${prefix_dir}/taskfactory" --version)" "taskfactory version ${expected_version}" 'PREFIX: installed binary reports the embedded version'
assert_contains "${output}" "taskfactory version ${expected_version}" 'PREFIX: install output prints the version'

gobin_dir="${dest_root}/gobin"
output=$(run_install GOBIN="${gobin_dir}")
test -x "${gobin_dir}/taskfactory"
assert_contains "${output}" "installed ${gobin_dir}/taskfactory" 'GOBIN: used when PREFIX is unset'

output=$(run_install PREFIX="${prefix_dir}/both" GOBIN="${gobin_dir}")
test -x "${prefix_dir}/both/taskfactory"
test ! -e "${gobin_dir}/taskfactory.unused"
assert_contains "${output}" "installed ${prefix_dir}/both/taskfactory" 'PREFIX takes precedence over GOBIN'

gopath_dir="${dest_root}/gopath"
# Reuse the caller's module cache so the temporary GOPATH holds no downloads.
output=$(run_install GOPATH="${gopath_dir}:${dest_root}/unused" GOMODCACHE="$(go env GOMODCACHE)")
test -x "${gopath_dir}/bin/taskfactory"
test ! -e "${dest_root}/unused"
assert_contains "${output}" "installed ${gopath_dir}/bin/taskfactory" 'falls back to the first GOPATH entry bin directory'

output=$(run_install PREFIX="${dest_root}/off-path")
assert_contains "${output}" 'is not on PATH' 'hints when the destination is not on PATH'

usage_status=0
usage_output=$(cd / && env -u PREFIX -u GOBIN -u GOPATH PREFIX="${dest_root}/rejected" "${install_script}" extra 2>&1) || usage_status=$?
assert_status "${usage_status}" 2 'rejects arguments with exit 2'
assert_contains "${usage_output}" 'Usage: bin/install' 'prints usage for extra arguments'
test ! -e "${dest_root}/rejected/taskfactory"
printf '[PASS] rejected arguments write nothing\n'

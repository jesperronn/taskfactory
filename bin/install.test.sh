#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"

known_scenarios="default idempotent optout foreign shadow help usage"
if (( $# > 0 )); then
  for requested in "$@"; do
    case " ${known_scenarios} " in
      *" ${requested} "*) ;;
      *) printf 'unknown scenario: %s\n' "${requested}" >&2; exit 2 ;;
    esac
  done
fi
selected() {
  local name=$1
  (( $# == 1 )) || true
  if (( ${#selected_list[@]} == 0 )); then return 0; fi
  local item
  for item in "${selected_list[@]}"; do [[ "${item}" == "${name}" ]] && return 0; done
  return 1
}
selected_list=("$@")

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
cp -R "${repo_root}/cmd" "${repo_root}/internal" "${repo_root}/skills" "${fixture}/"
git -C "${fixture}" init --quiet
git -C "${fixture}" add -A
git -C "${fixture}" -c user.name=test -c user.email=test@example.invalid commit --quiet -m fixture
expected_version=dev-$(git -C "${fixture}" rev-parse --short HEAD)
install_script="${fixture}/bin/install"

# run_install runs bin/install from / with PREFIX, GOBIN and GOPATH cleared,
# then applies the given NAME=value assignments.
run_install() {
  (cd / && env -u PREFIX -u GOBIN -u GOPATH "$@" "${install_script}" 2>&1)
}

dest_root="${fixture_root}/dest"

if selected default; then

prefix_dir="${dest_root}/prefix/nested"
output=$(run_install PREFIX="${prefix_dir}")
assert_contains "${output}" "installed ${prefix_dir}/taskfactory" 'PREFIX: prints the installed path'
test -x "${prefix_dir}/taskfactory"
printf '[PASS] PREFIX: installs an executable binary\n'
assert_contains "$("${prefix_dir}/taskfactory" --version)" "taskfactory version ${expected_version}" 'PREFIX: installed binary reports the embedded version'
assert_contains "${output}" "taskfactory version ${expected_version}" 'PREFIX: install output prints the version'
assert_contains "${output}" 'hint: enable tab completion with: source <(taskfactory completion bash)' 'PREFIX: install prints the completion hint'

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
fi

if selected default; then
  alias_dir="${dest_root}/alias-default"
  output=$(run_install PREFIX="${alias_dir}")
  test -L "${alias_dir}/tf"
  assert_status "$(readlink "${alias_dir}/tf")" taskfactory 'default: tf is a relative symlink to taskfactory'
  assert_status "$("${alias_dir}/tf" --version)" "$("${alias_dir}/taskfactory" --version)" 'default: tf --version equals taskfactory --version'
  printf 'default PASS\n'
fi

if selected idempotent; then
  alias_dir="${dest_root}/alias-idem"
  run_install PREFIX="${alias_dir}" >/dev/null
  run_install PREFIX="${alias_dir}" >/dev/null
  test -L "${alias_dir}/tf"
  assert_status "$(readlink "${alias_dir}/tf")" taskfactory 'idempotent: tf still points to taskfactory'
  printf 'idempotent PASS\n'
fi

if selected optout; then
  alias_dir="${dest_root}/alias-optout"
  run_install PREFIX="${alias_dir}" TASKFACTORY_NO_ALIAS=1 >/dev/null
  test -x "${alias_dir}/taskfactory"
  test ! -e "${alias_dir}/tf" && test ! -L "${alias_dir}/tf"
  printf 'optout PASS\n'
fi

if selected foreign; then
  alias_dir="${dest_root}/alias-foreign"
  mkdir -p "${alias_dir}"
  printf 'terraform\n' >"${alias_dir}/tf"
  status=0
  output=$(run_install PREFIX="${alias_dir}") || status=$?
  assert_status "${status}" 0 'foreign: file tf exits 0'
  assert_status "$(cat "${alias_dir}/tf")" terraform 'foreign: file tf untouched'
  assert_contains "${output}" "${alias_dir}/tf" 'foreign: warning names the path'
  test -x "${alias_dir}/taskfactory"
  alias_dir="${dest_root}/alias-foreign-link"
  mkdir -p "${alias_dir}"
  ln -s /usr/bin/true "${alias_dir}/tf"
  status=0
  output=$(run_install PREFIX="${alias_dir}") || status=$?
  assert_status "${status}" 0 'foreign: symlink tf exits 0'
  assert_status "$(readlink "${alias_dir}/tf")" /usr/bin/true 'foreign: symlink tf untouched'
  assert_contains "${output}" "${alias_dir}/tf" 'foreign: symlink warning names the path'
  printf 'foreign PASS\n'
fi

if selected shadow; then
  alias_dir="${dest_root}/alias-shadow"
  shadow_dir="${dest_root}/shadow-bin"
  mkdir -p "${shadow_dir}"
  printf '#!/bin/sh\nexit 0\n' >"${shadow_dir}/tf"
  chmod +x "${shadow_dir}/tf"
  output=$(run_install PREFIX="${alias_dir}" PATH="${shadow_dir}:${alias_dir}:${PATH}")
  assert_contains "${output}" "${shadow_dir}/tf" 'shadow: warning names the shadowing path'
  printf 'shadow PASS\n'
fi

if selected help; then
  alias_dir="${dest_root}/alias-help"
  run_install PREFIX="${alias_dir}" >/dev/null
  assert_status "$("${alias_dir}/tf" --help 2>&1)" "$("${alias_dir}/taskfactory" --help 2>&1)" 'help: tf --help equals taskfactory --help'
  printf 'help PASS\n'
fi

if selected usage; then
usage_status=0
usage_output=$(cd / && env -u PREFIX -u GOBIN -u GOPATH PREFIX="${dest_root}/rejected" "${install_script}" extra 2>&1) || usage_status=$?
assert_status "${usage_status}" 2 'rejects arguments with exit 2'
assert_contains "${usage_output}" 'Usage: bin/install' 'prints usage for extra arguments'
test ! -e "${dest_root}/rejected/taskfactory"
printf '[PASS] rejected arguments write nothing\n'
printf 'usage PASS\n'
fi

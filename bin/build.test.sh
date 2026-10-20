#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
scenarios=(tagged untagged dirty notag nogit)

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

# git_fixture creates a fixture that is its own Git repository with one commit.
git_fixture() {
  local name=$1
  local dir="${fixture_root}/${name}"
  new_fixture "${name}"
  git -C "${dir}" init --quiet
  git -C "${dir}" config commit.gpgsign false
  git -C "${dir}" config tag.gpgsign false
  git -C "${dir}" config user.name test
  git -C "${dir}" config user.email test@example.invalid
  git -C "${dir}" add -A
  git -C "${dir}" commit --quiet -m fixture
}

# check_version builds in the fixture and checks the printed and embedded version.
check_version() {
  local name=$1
  local expected=$2
  local dir="${fixture_root}/${name}"
  local build_output version_output
  build_output=$(cd / && "${dir}/bin/build" 2>&1)
  assert_contains "${build_output}" "built bin/out/taskfactory (version ${expected})" "${name} PASS build prints version ${expected}"
  version_output=$("${dir}/bin/out/taskfactory" --version)
  assert_contains "${version_output}" "taskfactory version ${expected}" "${name} PASS binary reports ${expected}"
}

scenario_tagged() {
  git_fixture tagged
  git -C "${fixture_root}/tagged" tag -a -m release v1.2.0
  check_version tagged v1.2.0
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

for name in "$@"; do
  "scenario_${name}"
done

#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"

# Tests for bin/setup. Hermetic: PATH holds only stubs and symlinks to the few
# utilities bin/setup uses, so no real go, git or perl is ever consulted.
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture=$(mktemp -d)
cleanup_test_tmp() { rm -rf "${fixture}"; }
trap cleanup_test_tmp EXIT

project="${fixture}/project"
mkdir -p "${project}/bin"
cp "${repo_root}/bin/setup" "${project}/bin/setup"
printf 'module x\n\ngo 1.23\n' > "${project}/go.mod"

util_dir="${fixture}/util"
mkdir -p "${util_dir}"
for util in awk grep head dirname; do
  ln -s "$(command -v "${util}")" "${util_dir}/${util}"
done

# mk_path NAME GO_VERSION TOOL...: a PATH dir with a go stub reporting
# GO_VERSION (empty: no go) and a stub for each TOOL.
mk_path() {
  local dir="${fixture}/$1" version=$2 tool
  shift 2
  mkdir -p "${dir}"
  if [[ -n "${version}" ]]; then
    # shellcheck disable=SC2016 # the stub script wants its own literal $ expansions
    printf '#!/bin/sh\necho "$*" >> "%s/go.calls"\n[ "$1" = version ] && echo "go version go%s linux/amd64"\nexit 0\n' \
      "${dir}" "${version}" > "${dir}/go"
    chmod +x "${dir}/go"
  fi
  for tool in "$@"; do
    printf '#!/bin/sh\nexit 0\n' > "${dir}/${tool}"
    chmod +x "${dir}/${tool}"
  done
  printf '%s\n' "${dir}"
}

run_setup() { # run_setup PATHDIR ARGS...: sets status and output
  local pdir=$1
  shift
  status=0
  output=$(cd "${project}" && PATH="${pdir}:${util_dir}" "${BASH}" bin/setup "$@" 2>&1) || status=$?
}

assert() { # assert DESCRIPTION CONDITION_STATUS
  if (( $2 != 0 )); then
    printf '[FAIL] %s: exit=%s output=%s\n' "$1" "${status}" "${output}" >&2
    exit 1
  fi
  printf '[PASS] %s\n' "$1"
}

good=$(mk_path good 1.25.11 git perl)

run_setup "${good}" --check
assert "--check passes with go 1.25.11, git and perl" "$(( status != 0 ))"
[[ ${output} == *"nothing changed"* ]] && rc=0 || rc=1
assert "--check says it changed nothing" "$rc"
calls=$(grep -c 'mod download' "${good}/go.calls" 2>/dev/null || true)
assert "--check never runs go mod download" "$(( calls != 0 ))"
[[ ${output} == *"skip    shellcheck"* ]] && rc=0 || rc=1
assert "missing shellcheck is only a skip" "$rc"

run_setup "${good}"
assert "setup exits 0 when the tools are present" "$(( status != 0 ))"
grep -q 'mod download' "${good}/go.calls"; assert "setup runs go mod download" "$?"
run_setup "${good}"
assert "setup is idempotent: a second run also exits 0" "$(( status != 0 ))"

old=$(mk_path old 1.24.2 git perl)
run_setup "${old}" --check
assert "go below the mdsmith minimum exits 1" "$(( status != 1 ))"
[[ ${output} == *"1.25.11"* ]] && rc=0 || rc=1
assert "the message names the needed Go version" "$rc"

older=$(mk_path older 1.22.0 git perl)
run_setup "${older}" --check
[[ ${status} == 1 && ${output} == *"go.mod"* ]] && rc=0 || rc=1
assert "go below go.mod's version names go.mod" "$rc"

nogit=$(mk_path nogit 1.26.0 perl)
run_setup "${nogit}" --check
[[ ${status} == 1 && ${output} == *"git not found"* ]] && rc=0 || rc=1
assert "missing git exits 1" "$rc"

nogo=$(mk_path nogo '' git perl)
run_setup "${nogo}" --check
[[ ${status} == 1 && ${output} == *"go not found"* ]] && rc=0 || rc=1
assert "missing go exits 1" "$rc"

run_setup "${good}" --bogus
assert "unknown option exits 2" "$(( status != 2 ))"

# Drift: the minimum Go in bin/setup is the one CI installs.
want=$(awk -F"'" '/go-version:/ { print $2; exit }' "${repo_root}/.github/workflows/ci.yml")
have=$(awk -F= '/^mdsmith_min_go=/ { print $2; exit }' "${repo_root}/bin/setup")
[[ ${want} == "${have}" && -n ${want} ]] && rc=0 || rc=1
assert "bin/setup minimum Go matches the CI go-version" "$rc"

#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture_root=$(mktemp -d)
trap 'rm -rf "${fixture_root}"' EXIT

# TF-106: a fixture can never reach the real go. GOFLAGS is invalid, so a real
# go fails at once, and every fixture gets a stubs/go that records the call and
# exits 99 on any call a scenario does not expect.
export GOFLAGS=-tf106-invalid-flag

# install_go_stub FIXTURE_DIR ALLOWED_CASE_PATTERN [BODY]: writes
# FIXTURE_DIR/stubs/go. It logs every call to ${GO_LOG}, runs BODY for an
# allowed call, and exits 99 naming the call for any call that does not match.
install_go_stub() {
  local dir=$1 allowed=$2 body=${3:-exit 0}
  mkdir -p "${dir}/stubs"
  cat > "${dir}/stubs/go" <<STUB
#!/usr/bin/env bash
printf '%s|%s\\n' "\$PWD" "\$*" >> "\${GO_LOG:-/dev/null}"
case "\$*" in
  ${allowed}) ;;
  *)
    printf 'stubs/go: unexpected go call: go %s\\n' "\$*" >&2
    exit 99
    ;;
esac
${body}
STUB
  chmod +x "${dir}/stubs/go"
}

new_fixture() {
  local name=$1
  mkdir -p "${fixture_root}/${name}/bin"
  cp "${repo_root}/bin/test" "${fixture_root}/${name}/bin/test"
  cp "${repo_root}/bin/testlib.sh" "${fixture_root}/${name}/bin/testlib.sh"
  install_go_stub "${fixture_root}/${name}" '"__none__"'
}

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

new_fixture no-go
cat > "${fixture_root}/no-go/bin/b.test.sh" <<'EOF'
#!/usr/bin/env bash
printf 'second\n' >> "${ORDER_FILE}"
EOF
cat > "${fixture_root}/no-go/bin/a.test.sh" <<'EOF'
#!/usr/bin/env bash
printf 'first\n' >> "${ORDER_FILE}"
EOF
chmod +x "${fixture_root}/no-go/bin/"*.test.sh
order_file=${fixture_root}/order
output=$(cd / && PATH="${fixture_root}/no-go/stubs:${PATH}" ORDER_FILE="${order_file}" "${fixture_root}/no-go/bin/test" 2>&1)
assert_contains "${output}" 'SKIP: Go tests (go.mod not found)' 'skips Go checks when go.mod is absent'
assert_contains "${output}" 'TEST SUMMARY: all selected checks passed' 'reports a passing shell-only fixture'
expected_order=$'first\nsecond'
actual_order=$(cat "${order_file}")
if [[ "${actual_order}" != "${expected_order}" ]]; then
  printf '[FAIL] stable shell test order: expected=%q actual=%q\n' "${expected_order}" "${actual_order}" >&2
  exit 1
fi
printf '[PASS] shell tests run in stable filename order from outside the repository\n'

new_fixture shell-failure
cat > "${fixture_root}/shell-failure/bin/fail.test.sh" <<'EOF'
#!/usr/bin/env bash
exit 7
EOF
chmod +x "${fixture_root}/shell-failure/bin/fail.test.sh"
failure_status=0
failure_output=$(cd / && PATH="${fixture_root}/shell-failure/stubs:${PATH}" "${fixture_root}/shell-failure/bin/test" 2>&1) || failure_status=$?
assert_status "${failure_status}" 1 'propagates shell test failure'
assert_contains "${failure_output}" 'FAIL: shell test: bin/fail.test.sh' 'identifies failing shell test'
assert_contains "${failure_output}" 'rerun with: ./bin/fail.test.sh' 'provides a valid direct rerun command'

new_fixture go-success
: > "${fixture_root}/go-success/go.mod"
mkdir -p "${fixture_root}/go-success/.taskfactory"
: > "${fixture_root}/go-success/.taskfactory/config.toml"
install_go_stub "${fixture_root}/go-success" 'test\ ./...|test\ ./pkg|run\ ./cmd/taskfactory\ validate\ *' 'exit "${GO_STATUS}"'
go_log=${fixture_root}/go-call
output=$(cd / && PATH="${fixture_root}/go-success/stubs:${PATH}" GO_LOG="${go_log}" GO_STATUS=0 "${fixture_root}/go-success/bin/test" 2>&1)
assert_contains "${output}" '==> go test ./...' 'runs Go checks when go.mod exists'
go_call=$(cat "${go_log}")
expected_go_call="${fixture_root}/go-success|test ./..."
if ! grep -Fqx "${expected_go_call}" "${go_log}"; then
  printf '[FAIL] Go command root and arguments: expected=%q actual=%q\n' "${expected_go_call}" "${go_call}" >&2
  exit 1
fi
if ! grep -Fqx "${fixture_root}/go-success|run ./cmd/taskfactory validate tasks" "${go_log}"; then
  printf '[FAIL] whole-tree validation command was not run: %s\n' "${go_call}" >&2
  exit 1
fi
printf '[PASS] Go tests and whole-tree validation run from fixture root\n'

new_fixture missing-config
: > "${fixture_root}/missing-config/go.mod"
missing_status=0
missing_output=$(cd / && PATH="${fixture_root}/missing-config/stubs:${PATH}" "${fixture_root}/missing-config/bin/test" 2>&1) || missing_status=$?
assert_status "${missing_status}" 1 'requires project config when a Go project is present'
assert_contains "${missing_output}" 'run taskfactory init' 'explains how to initialize missing task config'

new_fixture go-failure
: > "${fixture_root}/go-failure/go.mod"
mkdir -p "${fixture_root}/go-failure/stubs"
mkdir -p "${fixture_root}/go-failure/.taskfactory"
: > "${fixture_root}/go-failure/.taskfactory/config.toml"
cp "${fixture_root}/go-success/stubs/go" "${fixture_root}/go-failure/stubs/go"
failure_status=0
failure_output=$(cd / && PATH="${fixture_root}/go-failure/stubs:${PATH}" GO_LOG="${fixture_root}/go-failure-call" GO_STATUS=8 "${fixture_root}/go-failure/bin/test" 2>&1) || failure_status=$?
assert_status "${failure_status}" 1 'propagates Go test failure'
assert_contains "${failure_output}" 'FAIL: Go tests: go test ./...' 'identifies failing Go check'

new_fixture verbose
cat > "${fixture_root}/verbose/bin/a.test.sh" <<'EOF2'
#!/usr/bin/env bash
printf 'a:%s\n' "$*" >> "${ARGS_FILE}"
EOF2
cat > "${fixture_root}/verbose/bin/b.test.sh" <<'EOF2'
#!/usr/bin/env bash
printf 'b:%s\n' "$*" >> "${ARGS_FILE}"
EOF2
cat > "${fixture_root}/verbose/bin/testlib.sh" <<'EOF2'
printf 'testlib sourced\n' >> "${ARGS_FILE}"
EOF2
chmod +x "${fixture_root}/verbose/bin/"*.test.sh
args_file=${fixture_root}/verbose-args
rm -f "${args_file}"
(cd / && PATH="${fixture_root}/verbose/stubs:${PATH}" ARGS_FILE="${args_file}" "${fixture_root}/verbose/bin/test" --verbose >/dev/null 2>&1)
expected_verbose=$'a:--verbose\nb:--verbose'
if [[ "$(cat "${args_file}")" != "${expected_verbose}" ]]; then
  printf '[FAIL] --verbose is forwarded to each shell test and testlib.sh is not run: %q\n' "$(cat "${args_file}")" >&2
  exit 1
fi
printf '[PASS] --verbose is forwarded to each shell test, and sourced helpers are not run as tests\n'

rm -f "${args_file}"
(cd / && PATH="${fixture_root}/verbose/stubs:${PATH}" ARGS_FILE="${args_file}" "${fixture_root}/verbose/bin/test" >/dev/null 2>&1)
if [[ "$(cat "${args_file}")" != $'a:\nb:' ]]; then
  printf '[FAIL] shell tests run without arguments when --verbose is absent: %q\n' "$(cat "${args_file}")" >&2
  exit 1
fi
printf '[PASS] shell tests run without arguments when --verbose is absent\n'

usage_status=0
usage_output=$(cd / && PATH="${fixture_root}/verbose/stubs:${PATH}" "${fixture_root}/verbose/bin/test" --bogus 2>&1) || usage_status=$?
assert_status "${usage_status}" 2 'rejects unknown bin/test arguments'
assert_contains "${usage_output}" 'Usage: bin/test [--verbose]' 'prints usage for unknown arguments'

new_fixture paths
mkdir -p "${fixture_root}/paths/bin" "${fixture_root}/paths/pkg" "${fixture_root}/paths/stubs"
cat > "${fixture_root}/paths/bin/a.test.sh" <<'EOF2'
#!/usr/bin/env bash
printf 'a\n' >> "${ARGS_FILE}"
EOF2
cat > "${fixture_root}/paths/bin/b.test.sh" <<'EOF2'
#!/usr/bin/env bash
printf 'b\n' >> "${ARGS_FILE}"
EOF2
chmod +x "${fixture_root}/paths/bin/"*.test.sh
cat > "${fixture_root}/paths/bin/testlib.sh" <<'EOF2'
printf 'testlib\n' >> "${ARGS_FILE}"
EOF2
: > "${fixture_root}/paths/pkg/pkg.go"
cp "${fixture_root}/go-success/stubs/go" "${fixture_root}/paths/stubs/go"
paths_args=${fixture_root}/paths-args
paths_go=${fixture_root}/paths-go

# paths_run ARGS...: runs bin/test in the paths fixture, sets path_status and path_output.
paths_run() {
  rm -f "${paths_args}" "${paths_go}"
  path_status=0
  path_output=$(cd / && PATH="${fixture_root}/paths/stubs:${PATH}" ARGS_FILE="${paths_args}" GO_LOG="${paths_go}" GO_STATUS=0 "${fixture_root}/paths/bin/test" "$@" 2>&1) || path_status=$?
}

paths_run bin/a.test.sh
assert_status "${path_status}" 0 'a single shell test file runs alone'
if [[ "$(cat "${paths_args}")" != 'a' ]]; then
  printf '[FAIL] a single shell test file ran other tests: %q\n' "$(cat "${paths_args}")" >&2
  exit 1
fi
printf '[PASS] a single shell test file runs only that file\n'

paths_run bin/
assert_status "${path_status}" 0 'a folder runs its shell tests'
if [[ "$(cat "${paths_args}")" != $'a\nb' ]]; then
  printf '[FAIL] a folder runs its shell tests in order, skipping testlib.sh: %q\n' "$(cat "${paths_args}")" >&2
  exit 1
fi
printf '[PASS] a folder runs its shell tests in order and skips testlib.sh\n'

paths_run pkg
assert_status "${path_status}" 0 'a Go package folder runs go test for that package'
if ! grep -Fqx "${fixture_root}/paths|test ./pkg" "${paths_go}"; then
  printf '[FAIL] Go package folder did not run go test ./pkg: %q\n' "$(cat "${paths_go}")" >&2
  exit 1
fi
if [[ -e "${paths_args}" ]] || grep -Fq 'validate' "${paths_go}"; then
  printf '[FAIL] a Go package path ran shell tests or validation\n' >&2
  exit 1
fi
printf '[PASS] a Go package folder runs only go test and skips shell tests and validation\n'

paths_run './...'
assert_status "${path_status}" 0 'a Go package pattern runs go test for the pattern'
if ! grep -Fqx "${fixture_root}/paths|test ./..." "${paths_go}"; then
  printf '[FAIL] Go pattern did not run go test ./...: %q\n' "$(cat "${paths_go}")" >&2
  exit 1
fi
printf '[PASS] a Go package pattern runs go test with that pattern\n'

paths_run bin/missing.test.sh
assert_status "${path_status}" 2 'a missing shell test file exits 2'
assert_contains "${path_output}" 'no such shell test file: bin/missing.test.sh' 'a missing shell test file is named'

paths_run /etc
assert_status "${path_status}" 2 'an absolute path is refused'
assert_contains "${path_output}" 'path must be inside the repository' 'an absolute path explains the limit'

new_fixture budget
cat > "${fixture_root}/budget/bin/fast.test.sh" <<'EOF2'
#!/usr/bin/env bash
printf '[TIME]   0.100s [PASS] fast case\n'
printf '[TIME]   0.100s total\n'
EOF2
cat > "${fixture_root}/budget/bin/slow.test.sh" <<'EOF2'
#!/usr/bin/env bash
printf '[TIME]   2.500s [PASS] slow case\n'
printf '[TIME]   2.500s total\n'
EOF2
chmod +x "${fixture_root}/budget/bin/"*.test.sh
budget_status=0
budget_output=$(cd / && PATH="${fixture_root}/budget/stubs:${PATH}" "${fixture_root}/budget/bin/test" --budget bin/fast.test.sh 2>&1) || budget_status=$?
assert_status "${budget_status}" 0 '--budget passes a shell test with every case under 1s'
budget_status=0
budget_output=$(cd / && PATH="${fixture_root}/budget/stubs:${PATH}" "${fixture_root}/budget/bin/test" --budget bin/slow.test.sh 2>&1) || budget_status=$?
assert_status "${budget_status}" 1 '--budget fails a shell test with a case over 1s'
assert_contains "${budget_output}" 'budget: bin/slow.test.sh has cases over 1.000s' '--budget names the slow test file'
assert_contains "${budget_output}" '2.500s [PASS] slow case' '--budget prints the slow case'

# --budget parses go test -json and names a slow Go test.
new_fixture gobudget
: > "${fixture_root}/gobudget/go.mod"
mkdir -p "${fixture_root}/gobudget/pkg" "${fixture_root}/gobudget/stubs"
: > "${fixture_root}/gobudget/pkg/pkg.go"
cat > "${fixture_root}/gobudget/stubs/go" <<'EOF2'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${GO_LOG}"
if [[ "$*" != 'test -count=1 -json ./pkg' ]]; then
  printf 'stubs/go: unexpected go call: go %s\n' "$*" >&2
  exit 99
fi
printf '{"Action":"pass","Package":"x/pkg","Test":"TestSlowOne","Elapsed":%s}\n' "${SLOW_ELAPSED}"
printf '{"Action":"pass","Package":"x/pkg","Test":"TestFast","Elapsed":0.01}\n'
printf '{"Action":"pass","Package":"x/pkg","Elapsed":%s}\n' "${SLOW_ELAPSED}"
EOF2
chmod +x "${fixture_root}/gobudget/stubs/go"
gobudget_run() {
  budget_status=0
  budget_output=$(cd / && PATH="${fixture_root}/gobudget/stubs:${PATH}" GO_LOG="${fixture_root}/gobudget-go" SLOW_ELAPSED="$1" "${fixture_root}/gobudget/bin/test" --budget pkg 2>&1) || budget_status=$?
}
gobudget_run 0.02
assert_status "${budget_status}" 0 '--budget passes fast Go tests'
assert_contains "$(cat "${fixture_root}/gobudget-go")" 'test -count=1 -json ./pkg' '--budget runs go test -json'
gobudget_run 6.5
assert_status "${budget_status}" 1 '--budget fails Go tests over 5s in total'
assert_contains "${budget_output}" 'budget: Go tests took 6500ms, over the 5000ms limit' '--budget reports the Go total'
assert_contains "${budget_output}" '6.50s x/pkg TestSlowOne' '--budget names the slow Go test and its time'

# isolation: a bare fixture (only bin/test and bin/testlib.sh, no scenario stub)
# never starts go test, and the real go is unreachable.
new_fixture isolation
iso=${fixture_root}/isolation
iso_path="${iso}/stubs:/usr/bin:/bin"
if [[ "$(PATH="${iso_path}" command -v go)" != "${iso}/stubs/go" ]]; then
  printf '[FAIL] isolation: go does not resolve to the fixture stub\n' >&2
  exit 1
fi
if [[ -n "$(find "${iso}" -name '*.test.sh')" ]]; then
  printf '[FAIL] isolation: fixture holds real shell tests\n' >&2
  exit 1
fi
printf '[PASS] isolation: fixture copies only bin/test and bin/testlib.sh and resolves go to the stub\n'
iso_status=0
iso_output=$(cd / && PATH="${iso_path}" GO_LOG="${iso}/go-calls" "${iso}/bin/test" 2>&1) || iso_status=$?
assert_status "${iso_status}" 0 'isolation: a bare fixture exits without starting go test'
assert_contains "${iso_output}" 'SKIP: Go tests (go.mod not found)' 'isolation: a bare fixture skips Go checks'
if [[ -e "${iso}/go-calls" ]]; then
  printf '[FAIL] isolation: a bare fixture called go: %q\n' "$(cat "${iso}/go-calls")" >&2
  exit 1
fi
printf '[PASS] isolation: a bare fixture records no go call\n'
: > "${iso}/go.mod"
iso_status=0
iso_output=$(cd / && PATH="${iso_path}" GO_LOG="${iso}/go-calls" "${iso}/bin/test" 2>&1) || iso_status=$?
assert_status "${iso_status}" 1 'isolation: an unexpected go call fails the fixture'
assert_contains "${iso_output}" 'stubs/go: unexpected go call: go test ./...' 'isolation: the stub names the unexpected go call'
assert_contains "${iso_output}" 'FAIL: Go tests: go test ./...' 'isolation: the failed go test is reported, not run'
assert_contains "$(cat "${iso}/go-calls")" '|test ./...' 'isolation: the stub recorded the unexpected call'

#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture_root=$(mktemp -d)
trap 'rm -rf "${fixture_root}"' EXIT

new_fixture() {
  local name=$1
  mkdir -p "${fixture_root}/${name}/bin"
  cp "${repo_root}/bin/test" "${fixture_root}/${name}/bin/test"
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
output=$(cd / && ORDER_FILE="${order_file}" "${fixture_root}/no-go/bin/test" 2>&1)
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
failure_output=$(cd / && "${fixture_root}/shell-failure/bin/test" 2>&1) || failure_status=$?
assert_status "${failure_status}" 1 'propagates shell test failure'
assert_contains "${failure_output}" 'FAIL: shell test: bin/fail.test.sh' 'identifies failing shell test'
assert_contains "${failure_output}" 'rerun with: ./bin/fail.test.sh' 'provides a valid direct rerun command'

new_fixture go-success
: > "${fixture_root}/go-success/go.mod"
mkdir -p "${fixture_root}/go-success/stubs"
mkdir -p "${fixture_root}/go-success/.taskfactory"
: > "${fixture_root}/go-success/.taskfactory/config.toml"
cat > "${fixture_root}/go-success/stubs/go" <<'EOF'
#!/usr/bin/env bash
printf '%s|%s\n' "$PWD" "$*" >> "${GO_LOG}"
exit "${GO_STATUS}"
EOF
chmod +x "${fixture_root}/go-success/stubs/go"
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
missing_output=$(cd / && "${fixture_root}/missing-config/bin/test" 2>&1) || missing_status=$?
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
(cd / && ARGS_FILE="${args_file}" "${fixture_root}/verbose/bin/test" --verbose >/dev/null 2>&1)
expected_verbose=$'a:--verbose\nb:--verbose'
if [[ "$(cat "${args_file}")" != "${expected_verbose}" ]]; then
  printf '[FAIL] --verbose is forwarded to each shell test and testlib.sh is not run: %q\n' "$(cat "${args_file}")" >&2
  exit 1
fi
printf '[PASS] --verbose is forwarded to each shell test, and sourced helpers are not run as tests\n'

rm -f "${args_file}"
(cd / && ARGS_FILE="${args_file}" "${fixture_root}/verbose/bin/test" >/dev/null 2>&1)
if [[ "$(cat "${args_file}")" != $'a:\nb:' ]]; then
  printf '[FAIL] shell tests run without arguments when --verbose is absent: %q\n' "$(cat "${args_file}")" >&2
  exit 1
fi
printf '[PASS] shell tests run without arguments when --verbose is absent\n'

usage_status=0
usage_output=$(cd / && "${fixture_root}/verbose/bin/test" --bogus 2>&1) || usage_status=$?
assert_status "${usage_status}" 2 'rejects unknown bin/test arguments'
assert_contains "${usage_output}" 'Usage: bin/test [--verbose]' 'prints usage for unknown arguments'

#!/usr/bin/env bash
set -euo pipefail

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
cat > "${fixture_root}/go-success/stubs/go" <<'EOF'
#!/usr/bin/env bash
printf '%s|%s\n' "$PWD" "$*" > "${GO_LOG}"
exit "${GO_STATUS}"
EOF
chmod +x "${fixture_root}/go-success/stubs/go"
go_log=${fixture_root}/go-call
output=$(cd / && PATH="${fixture_root}/go-success/stubs:${PATH}" GO_LOG="${go_log}" GO_STATUS=0 "${fixture_root}/go-success/bin/test" 2>&1)
assert_contains "${output}" '==> go test ./...' 'runs Go checks when go.mod exists'
go_call=$(cat "${go_log}")
expected_go_call="${fixture_root}/go-success|test ./..."
if [[ "${go_call}" != "${expected_go_call}" ]]; then
  printf '[FAIL] Go command root and arguments: expected=%q actual=%q\n' "${expected_go_call}" "${go_call}" >&2
  exit 1
fi
printf '[PASS] Go tests run from fixture root with go test ./...\n'

new_fixture go-failure
: > "${fixture_root}/go-failure/go.mod"
mkdir -p "${fixture_root}/go-failure/stubs"
cp "${fixture_root}/go-success/stubs/go" "${fixture_root}/go-failure/stubs/go"
failure_status=0
failure_output=$(cd / && PATH="${fixture_root}/go-failure/stubs:${PATH}" GO_LOG="${fixture_root}/go-failure-call" GO_STATUS=8 "${fixture_root}/go-failure/bin/test" 2>&1) || failure_status=$?
assert_status "${failure_status}" 1 'propagates Go test failure'
assert_contains "${failure_output}" 'FAIL: Go tests: go test ./...' 'identifies failing Go check'

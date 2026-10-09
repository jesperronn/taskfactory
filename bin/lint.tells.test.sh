#!/usr/bin/env bash
set -euo pipefail

# Tests for the advisory `bin/lint --tells` mode. They run the real pinned
# mdsmith against a scratch project that holds one file with known tells.
cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_dir=$PWD
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

project="$fixture/project"
mkdir -p "$project/bin" "$project/docs"
cp "$repo_dir/bin/lint" "$project/bin/lint"
cp "$repo_dir/.mdsmith.yml" "$repo_dir/.mdsmith-tells.yml" "$project/"
printf '# Title\n\nWe delve into the plan.\n\nMoreover, it works.\n' \
  > "$project/docs/tells.md"

run_lint() {  # run_lint ARGS...: sets status and output
  status=0
  output=$(cd "$project" && "$project/bin/lint" "$@" 2>&1) || status=$?
}

fail() {
  printf '[FAIL] %s: exit=%s output=%s\n' "$1" "$status" "$output" >&2
  exit 1
}

run_lint --tells
if (( status != 0 )); then fail "tells exits 0 with findings"; fi
if [[ $output != *"docs/tells.md"*MDS056*delve* ]]; then fail "tells reports MDS056"; fi
if [[ $output != *MDS055*Moreover* ]]; then fail "tells reports MDS055"; fi
printf '[PASS] --tells exits 0 and prints the findings\n'

if [[ $output != *"stats: tells MDS055=1 MDS056=1"* ]]; then fail "tells stats line"; fi
printf '[PASS] --tells prints the stats line\n'

# The default lint must never pass the tells config to mdsmith.

mkdir -p "$fixture/stub"
cat > "$fixture/stub/go" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$LINT_ARGS_FILE"
exit 0
STUB
chmod +x "$fixture/stub/go"
args="$fixture/args"
: > "$args"
status=0
output=$(cd "$project" && PATH="$fixture/stub:$PATH" LINT_ARGS_FILE="$args" \
  "$project/bin/lint" docs/tells.md 2>&1) || status=$?
if (( status != 0 )) || grep -q -- 'mdsmith-tells' "$args"; then
  fail "default lint ignores the tells config"
fi
printf '[PASS] default mode exits 0 and never passes the tells config\n'

run_lint --tells --autofix
if (( status != 2 )); then fail "tells with --autofix"; fi
run_lint --tells docs/tells.md
if (( status != 2 )); then fail "tells with a path"; fi
printf '[PASS] --tells usage errors exit 2\n'

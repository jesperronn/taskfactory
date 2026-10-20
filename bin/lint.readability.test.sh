#!/usr/bin/env bash
set -euo pipefail

# Tests for the advisory `bin/lint --readability` mode. They run the real
# pinned mdsmith against a scratch project that holds one hard paragraph.
cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_dir=$PWD
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

project="$fixture/project"
mkdir -p "$project/bin" "$project/docs"
cp "$repo_dir/bin/lint" "$project/bin/lint"
cp "$repo_dir/.mdsmith.yml" "$repo_dir/.mdsmith-readability.yml" "$project/"
{
  printf '# Title\n\n'
  printf 'The implementation of concurrent distributed systems requires '
  printf 'sophisticated understanding of fundamental computational paradigms '
  printf 'and synchronization mechanisms that must guarantee linearizability '
  printf 'across heterogeneous processing environments.\n'
} > "$project/docs/worker-instructions.md"

run_lint() {  # run_lint ARGS...: sets status and output
  status=0
  output=$(cd "$project" && "$project/bin/lint" "$@" 2>&1) || status=$?
}

fail() {
  printf '[FAIL] %s: exit=%s output=%s\n' "$1" "$status" "$output" >&2
  exit 1
}

run_lint --readability
if (( status != 0 )); then fail "readability exits 0 with findings"; fi
if [[ $output != *"docs/worker-instructions.md"*MDS023* ]]; then
  fail "readability reports MDS023"
fi
if [[ $output != *"stats: readability MDS023=1 MDS024=0 MDS036=0"* ]]; then
  fail "readability stats line"
fi
printf '[PASS] --readability exits 0, prints findings and the stats line\n'

# The default lint must never pass the readability config to mdsmith.

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
  "$project/bin/lint" docs/worker-instructions.md 2>&1) || status=$?
if (( status != 0 )) || grep -q -- 'mdsmith-readability' "$args"; then
  fail "default lint ignores the readability config"
fi
printf '[PASS] default mode exits 0 and never passes the readability config\n'

run_lint --readability --autofix
if (( status != 2 )); then fail "readability with --autofix"; fi
run_lint --readability docs/worker-instructions.md
if (( status != 2 )); then fail "readability with a path"; fi
printf '[PASS] --readability usage errors exit 2\n'

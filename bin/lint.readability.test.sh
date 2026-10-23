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

printf '# Other\n\n' > "$project/docs/other.md"
cat "$project/docs/worker-instructions.md" >> "$project/docs/other.md"

# A stub mdsmith records its arguments, so the test sees which config each
# default-lint run passes. It prints a stats line, as the real mdsmith does.
mkdir -p "$fixture/stub"
cat > "$fixture/stub/go" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$LINT_ARGS_FILE"
echo "stats: checked=1 fixed=0 failures=0 unfixed=0"
exit 0
STUB
chmod +x "$fixture/stub/go"
args="$fixture/args"

# Only the gated worker-facing files get the readability config.
: > "$args"
status=0
output=$(cd "$project" && PATH="$fixture/stub:$PATH" LINT_ARGS_FILE="$args" \
  "$project/bin/lint" docs/other.md 2>&1) || status=$?
if (( status != 0 )) || grep -q -- 'mdsmith-readability' "$args"; then
  fail "default lint leaves other Markdown advisory"
fi
printf '[PASS] default lint on other Markdown never passes the readability config\n'

: > "$args"
status=0
output=$(cd "$project" && PATH="$fixture/stub:$PATH" LINT_ARGS_FILE="$args" \
  "$project/bin/lint" docs/worker-instructions.md 2>&1) || status=$?
if (( status != 0 )) || ! grep -q -- '-c .mdsmith-readability.yml docs/worker-instructions.md' "$args"; then
  fail "default lint gates the worker instructions"
fi
printf '[PASS] default lint passes the readability config for a gated file\n'

# The real mdsmith: a long sentence fails the default lint in a worker-facing
# file and passes in other Markdown. The paragraph is wrapped so that
# line-length does not decide the result.
gate="$fixture/gate"
mkdir -p "$gate/bin" "$gate/docs"
cp "$repo_dir/bin/lint" "$gate/bin/lint"
cp "$repo_dir/.mdsmith.yml" "$repo_dir/.mdsmith-readability.yml" "$gate/"
{
  printf '# Title\n\n'
  printf '%s\n' 'The implementation of concurrent distributed systems requires sophisticated understanding of fundamental computational paradigms and synchronization mechanisms that must guarantee linearizability across heterogeneous processing environments.' | fold -s -w 78
} > "$gate/docs/worker-instructions.md"
cp "$gate/docs/worker-instructions.md" "$gate/docs/other.md"

run_gate() {  # run_gate ARGS...: sets status and output in the gate project
  status=0
  output=$(cd "$gate" && "$gate/bin/lint" "$@" 2>&1) || status=$?
}

run_gate docs/worker-instructions.md
if (( status != 1 )) || [[ $output != *"docs/worker-instructions.md"*MDS023* ]]; then
  fail "default lint fails a long sentence in a worker-facing file"
fi
printf '[PASS] default lint fails a long sentence in a worker-facing file\n'

run_gate docs/other.md
if (( status != 0 )); then fail "default lint passes a long sentence in other Markdown"; fi
printf '[PASS] default lint passes a long sentence in other Markdown\n'

run_lint --readability --autofix
if (( status != 2 )); then fail "readability with --autofix"; fi
run_lint --readability docs/worker-instructions.md
if (( status != 2 )); then fail "readability with a path"; fi
printf '[PASS] --readability usage errors exit 2\n'

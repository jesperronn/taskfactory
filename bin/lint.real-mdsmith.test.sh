#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_dir=$PWD
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

mkdir -p "$fixture/project/bin" "$fixture/project/docs"
cp "$repo_dir/bin/lint" "$fixture/project/bin/lint"
cp "$repo_dir/.mdsmith.yml" "$fixture/project/.mdsmith.yml"

# Two blank lines between blocks violate no-multiple-blanks (MDS008).
printf '# Title\n\n\nBad spacing.\n' > "$fixture/project/docs/bad.md"
bad_status=0
bad_output=$("$fixture/project/bin/lint" 2>&1) || bad_status=$?
if (( bad_status != 1 )) || [[ $bad_output != *docs/bad.md* ]]; then
  printf '[FAIL] misformatted Markdown: exit=%s output=%s\n' "$bad_status" "$bad_output" >&2
  exit 1
fi
printf '[PASS] misformatted Markdown exits %s and names docs/bad.md\n' "$bad_status"

fix_status=0
fix_output=$("$fixture/project/bin/lint" --autofix 2>&1) || fix_status=$?
if (( fix_status != 0 )); then
  printf '[FAIL] --autofix on misformatted Markdown: exit=%s output=%s\n' "$fix_status" "$fix_output" >&2
  exit 1
fi
printf '[PASS] --autofix repairs misformatted Markdown and exits 0\n'

rm "$fixture/project/docs/bad.md"
printf '# Title\n\nGood spacing.\n' > "$fixture/project/docs/good.md"
good_status=0
good_output=$("$fixture/project/bin/lint" 2>&1) || good_status=$?
if (( good_status != 0 )); then
  printf '[FAIL] formatted Markdown: exit=%s output=%s\n' "$good_status" "$good_output" >&2
  exit 1
fi
printf '[PASS] formatted Markdown exits 0\n'

#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_dir=$PWD
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

mkdir -p "$fixture/project/bin" "$fixture/project/docs"
cp "$repo_dir/bin/lint" "$fixture/project/bin/lint"

prettier_version=$(npx prettier --version)

printf '# Title\n\n\nBad  spacing.  \n' > "$fixture/project/docs/bad.md"
bad_status=0
bad_output=$("$fixture/project/bin/lint" 2>&1) || bad_status=$?
if (( bad_status == 0 )) || [[ $bad_output != *docs/bad.md* ]]; then
  printf '[FAIL] misformatted Markdown: exit=%s output=%s\n' "$bad_status" "$bad_output" >&2
  exit 1
fi
printf '[PASS] misformatted Markdown exits %s and names docs/bad.md\n' "$bad_status"

rm "$fixture/project/docs/bad.md"
printf '# Title\n\nGood spacing.\n' > "$fixture/project/docs/good.md"
good_status=0
good_output=$("$fixture/project/bin/lint" 2>&1) || good_status=$?
if (( good_status != 0 )) || [[ $good_output == *'[warn]'* ]]; then
  printf '[FAIL] formatted Markdown: exit=%s output=%s\n' "$good_status" "$good_output" >&2
  exit 1
fi
printf '[PASS] formatted Markdown exits 0 with no formatting warnings\n'
printf 'Prettier %s\n' "$prettier_version"

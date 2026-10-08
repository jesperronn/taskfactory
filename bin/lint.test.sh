#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_dir=$PWD
stub_dir=$(mktemp -d)
trap 'rm -rf "$stub_dir"' EXIT

cat > "$stub_dir/npx" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$LINT_ARGS_FILE"
exit "$LINT_STUB_EXIT"
EOF
chmod +x "$stub_dir/npx"

args_file="$stub_dir/args"
for mode in check autofix; do
  for expected_status in 0 1; do
    if [[ $mode == autofix ]]; then
      option=--autofix
      expected_args='prettier --write --prose-wrap always **/*.md'
    else
      option=
      expected_args='prettier --check --prose-wrap always **/*.md'
    fi

    actual_status=0
    PATH="$stub_dir:$PATH" LINT_ARGS_FILE="$args_file" LINT_STUB_EXIT="$expected_status" \
      "$repo_dir/bin/lint" ${option:+"$option"} || actual_status=$?
    actual_args=$(cat "$args_file")

    if [[ $actual_status != "$expected_status" || $actual_args != "$expected_args" ]]; then
      printf '[FAIL] %s: expected status=%s args=%q; actual status=%s args=%q\n' \
        "$mode" "$expected_status" "$expected_args" "$actual_status" "$actual_args" >&2
      exit 1
    fi
    printf '[PASS] %s exits %s and passes expected arguments\n' "$mode" "$actual_status"
  done
done

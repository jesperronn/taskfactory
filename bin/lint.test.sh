#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_dir=$PWD
stub_dir=$(mktemp -d)
trap 'rm -rf "$stub_dir"' EXIT

cat > "$stub_dir/go" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" > "$LINT_ARGS_FILE"
exit "$LINT_STUB_EXIT"
EOF
chmod +x "$stub_dir/go"

mdsmith='run github.com/jeduden/mdsmith/cmd/mdsmith@v0.57.0'
args_file="$stub_dir/args"
for mode in check autofix; do
  for expected_status in 0 1 2; do
    if [[ $mode == autofix ]]; then
      option=--autofix
      expected_args="$mdsmith fix"
    else
      option=
      expected_args="$mdsmith check"
    fi
    # The stub stands in for mdsmith's own exit codes: 0, 1 and 2 must pass through.

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

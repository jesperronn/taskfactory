#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_dir=$PWD
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

# The fixture is a scratch copy of the module and task tree, so the validate
# half runs against real contracts and real dependencies.
project="$fixture/project"
mkdir -p "$project"
cp -R "$repo_dir/bin" "$repo_dir/cmd" "$repo_dir/internal" "$repo_dir/tasks" \
  "$repo_dir/go.mod" "$repo_dir/go.sum" "$repo_dir/.mdsmith.yml" "$project/"
if [[ -d $repo_dir/.taskfactory ]]; then
  cp -R "$repo_dir/.taskfactory" "$project/"
fi
# The validator needs a Git working tree; the fixture only needs the directory.
git -C "$project" init -q

task_rel=tasks/ready/TF-041-lint-tasks-with-validate.md
pristine="$repo_dir/$task_rel"
target="$project/$task_rel"

# misformat: a second blank line after the title breaks no-multiple-blanks
# (MDS008) and leaves the contract intact.
misformat() {
  { head -n 1 "$pristine"; printf '\n\n'; tail -n +2 "$pristine"; } > "$target"
}

# break_contract: a dependency on a task that does not exist fails validation
# and leaves the Markdown intact.
break_contract() {
  sed 's/^- TF-045$/- TF-999/' "$pristine" > "$target"
}

run_lint() {  # run_lint ARGS...: sets status and output
  status=0
  output=$(cd "$project" && "$project/bin/lint" "$@" 2>&1) || status=$?
}

fail() {
  printf '[FAIL] %s: exit=%s output=%s\n' "$1" "$status" "$output" >&2
  exit 1
}

# Lint half fails: exit 1, the file is named as a Markdown failure, and the
# validate half still ran and passed.
misformat
run_lint "$task_rel"
if (( status != 1 )) || [[ $output != *"FAIL markdown lint: $task_rel"* ]] \
  || [[ $output == *"FAIL task validation"* ]] \
  || [[ $output != *"1 task file(s) valid"* ]]; then
  fail "misformatted task file (lint-fail)"
fi
printf '[PASS] misformatted task fails the lint half and passes validation\n'

# Validate half fails: exit 1, the file is named as a validation failure, and
# the Markdown lint still ran and passed.
break_contract
run_lint "$task_rel"
if (( status != 1 )) || [[ $output != *"FAIL task validation: $task_rel"* ]] \
  || [[ $output == *"FAIL markdown lint"* ]] \
  || [[ $output != *"TF-999"* ]]; then
  fail "broken task contract (validate-fail)"
fi
printf '[PASS] broken contract fails the validate half and passes lint\n'

# Clean task: both halves pass.
cp "$pristine" "$target"
run_lint "$task_rel"
if (( status != 0 )) || [[ $output == *"FAIL"* ]] \
  || [[ $output != *"1 task file(s) valid"* ]]; then
  fail "clean task file"
fi
printf '[PASS] clean task passes both halves\n'

# Both broken: both failures are named, and neither stops the other.
break_contract
{ head -n 1 "$target"; printf '\n\n'; tail -n +2 "$target"; } > "$target.tmp"
mv "$target.tmp" "$target"
run_lint "$task_rel"
if (( status != 1 )) || [[ $output != *"FAIL markdown lint: $task_rel"* ]] \
  || [[ $output != *"FAIL task validation: $task_rel"* ]]; then
  fail "both halves broken"
fi
printf '[PASS] both halves broken names both failures\n'

# Archive: tasks/archive is not validated. The copy below has a broken contract
# but clean Markdown, so a validate run would fail and exit 1.
archive_rel=tasks/archive/TF-990-archived-copy.md
sed -e 's/TF-041/TF-990/g' -e 's/^- TF-045$/- TF-999/' "$pristine" \
  > "$project/$archive_rel"
run_lint "$archive_rel"
if (( status != 0 )) || [[ $output == *"task validation"* ]]; then
  fail "archived task file is validated"
fi
printf '[PASS] archived task file is linted but not validated\n'

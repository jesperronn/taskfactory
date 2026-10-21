#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"

cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_dir=$PWD

# tasksdir scenario: a project whose config sets tasks_dir under .taskfactory.
# It runs the real, pinned mdsmith with the repository's .mdsmith.yml, so it
# proves that Markdown under .taskfactory/tasks is linted while a worktree copy
# under .taskfactory/worktrees stays ignored. Only the taskfactory and go vet
# calls are recorded instead of run.
run_tasksdir_scenario() {
  local real_go dir project stubs status output tf_file
  real_go=$(command -v go)
  dir=$(mktemp -d)
  stubs=$dir/stubs
  project=$dir/project
  tf_file=$dir/taskfactory
  mkdir -p "$stubs" "$project/bin" "$project/.taskfactory/tasks/ready" \
    "$project/.taskfactory/tasks/archive" "$project/.taskfactory/worktrees/TF-1/docs"
  cat > "$stubs/go" <<EOF
#!/usr/bin/env bash
case "\$*" in
  run\ github.com/jeduden/mdsmith/*) exec "$real_go" "\$@" ;;
  vet\ *) exit 0 ;;
  run\ ./cmd/taskfactory\ *) printf '%s\n' "\$*" >> "$tf_file"; exit 0 ;;
  *) echo "unexpected go call: \$*" >&2; exit 99 ;;
esac
EOF
  chmod +x "$stubs/go"
  cp "$repo_dir/bin/lint" "$project/bin/lint"
  cp "$repo_dir/.mdsmith.yml" "$project/.mdsmith.yml"
  : > "$project/go.mod"
  printf 'tasks_dir = ".taskfactory/tasks"\n\n[workers]\nmax_parallel = 1\n' > "$project/.taskfactory/config.toml"
  local task=.taskfactory/tasks/ready/TF-900-x.md
  printf '# TF-900: x\n\n\nBad spacing.\n' > "$project/$task"
  printf '# Title\n\n\nBad spacing.\n' > "$project/.taskfactory/worktrees/TF-1/docs/bad.md"
  printf '# TF-901: old\n\nFine.\n' > "$project/.taskfactory/tasks/archive/TF-901-old.md"

  lint_here() {  # lint_here ARGS...: sets status and output
    status=0
    output=$(cd "$project" && PATH="$stubs:$PATH" bin/lint "$@" 2>&1) || status=$?
  }
  tasksdir_fail() {
    printf '[FAIL] tasksdir %s: exit=%s output=%s\n' "$1" "$status" "$output" >&2
    rm -rf "$dir"
    exit 1
  }

  lint_here
  if (( status == 0 )) || [[ $output != *"$task"* ]]; then tasksdir_fail "whole project must lint $task"; fi
  if [[ $output == *worktrees* ]]; then tasksdir_fail "worktree copy must stay ignored"; fi
  printf '[PASS] tasksdir: whole-project lint reports Markdown under .taskfactory/tasks\n'

  printf '# TF-900: x\n\nGood spacing.\n' > "$project/$task"
  : > "$tf_file"
  lint_here
  if (( status != 0 )); then tasksdir_fail "whole project with a clean task file"; fi
  if [[ $(cat "$tf_file") != "run ./cmd/taskfactory validate .taskfactory/tasks" ]]; then
    tasksdir_fail "whole-tree validation args=$(cat "$tf_file")"
  fi
  printf '[PASS] tasksdir: worktree copies stay ignored and the whole tree validates .taskfactory/tasks\n'

  printf '# TF-900: x\n\n\nBad spacing.\n' > "$project/$task"
  : > "$tf_file"
  lint_here "$task"
  if (( status == 0 )) || [[ $output != *"FAIL markdown lint: $task"* ]]; then tasksdir_fail "task path markdown lint"; fi
  if [[ $(cat "$tf_file") != "run ./cmd/taskfactory validate $task" ]]; then
    tasksdir_fail "task path validation args=$(cat "$tf_file")"
  fi
  printf '[PASS] tasksdir: a task file path gets the Markdown lint and contract validation\n'

  printf '# TF-901: old\n\n\nBad spacing.\n' > "$project/.taskfactory/tasks/archive/TF-901-old.md"
  : > "$tf_file"
  lint_here .taskfactory/tasks/archive/TF-901-old.md
  if [[ -s $tf_file ]]; then tasksdir_fail "archive files must not be contract-checked"; fi
  printf '[PASS] tasksdir: archived files are not contract-checked\n'
  rm -rf "$dir"
}

scenario=${1:-all}
case "$scenario" in
  all) ;;
  tasksdir) run_tasksdir_scenario; exit 0 ;;
  *) echo "usage: bin/lint.test.sh [--verbose] [tasksdir]" >&2; exit 2 ;;
esac

stub_dir=$(mktemp -d)
trap 'rm -rf "$stub_dir"' EXIT

# The go stub records mdsmith and go vet invocations separately. Only the
# mdsmith call takes LINT_STUB_EXIT, so its exit codes pass through unchanged.
cat > "$stub_dir/go" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  run\ github.com/jeduden/mdsmith/*)
    printf '%s\n' "$*" > "$LINT_ARGS_FILE"
    exit "$LINT_STUB_EXIT" ;;
  vet\ *)
    printf '%s\n' "$*" > "$LINT_VET_FILE"
    exit 0 ;;
  run\ ./cmd/taskfactory\ *)
    printf '%s\n' "$*" > "$LINT_TF_FILE"
    exit 0 ;;
  *) echo "unexpected go call: $*" >&2; exit 99 ;;
esac
EOF
chmod +x "$stub_dir/go"

mdsmith='run github.com/jeduden/mdsmith/cmd/mdsmith@v0.57.0'
args_file="$stub_dir/args"
vet_file="$stub_dir/vet"
tf_file="$stub_dir/taskfactory"
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
    rm -f "$args_file" "$vet_file" "$tf_file"
    PATH="$stub_dir:$PATH" LINT_ARGS_FILE="$args_file" LINT_VET_FILE="$vet_file" \
      LINT_TF_FILE="$tf_file" LINT_STUB_EXIT="$expected_status" \
      "$repo_dir/bin/lint" ${option:+"$option"} || actual_status=$?
    actual_args=$(cat "$args_file")
    actual_vet=$(cat "$vet_file")
    actual_tf=$(cat "$tf_file")
    if [[ $actual_tf != "run ./cmd/taskfactory validate tasks" ]]; then
      printf '[FAIL] %s: whole-tree validation args=%q\n' "$mode" "$actual_tf" >&2
      exit 1
    fi

    if [[ $actual_status != "$expected_status" || $actual_args != "$expected_args" ]]; then
      printf '[FAIL] %s: expected status=%s args=%q; actual status=%s args=%q\n' \
        "$mode" "$expected_status" "$expected_args" "$actual_status" "$actual_args" >&2
      exit 1
    fi
    if [[ $actual_vet != "vet ./..." ]]; then
      printf '[FAIL] %s: whole-project go vet args=%q, want "vet ./..."\n' \
        "$mode" "$actual_vet" >&2
      exit 1
    fi
    printf '[PASS] %s exits %s, passes expected arguments, and runs go vet ./...\n' \
      "$mode" "$actual_status"
  done
done

# Path arguments: a misformatted Go file must fail, and a path argument must
# lint only that path.
fixture=$(mktemp -d)
trap 'rm -rf "$stub_dir" "$fixture"' EXIT
mkdir -p "$fixture/project/bin" "$fixture/project/docs"
cp "$repo_dir/bin/lint" "$fixture/project/bin/lint"
cp "$repo_dir/.mdsmith.yml" "$fixture/project/.mdsmith.yml"
printf 'package main\n\nfunc main()  {}\n' > "$fixture/project/bad.go"
printf 'package main\n\nfunc main() {}\n' > "$fixture/project/good.go"
printf '# Title\n\nGood spacing.\n' > "$fixture/project/docs/good.md"
printf '# Title\n\n\nBad spacing.\n' > "$fixture/project/docs/bad.md"
lint="$fixture/project/bin/lint"

run_lint() {  # run_lint ARGS...: sets status and output
  status=0
  output=$(cd "$fixture/project" && PATH="$stub_dir:$PATH" LINT_ARGS_FILE="$stub_dir/args" \
    LINT_VET_FILE="$stub_dir/vet" LINT_STUB_EXIT=0 "$lint" "$@" 2>&1) || status=$?
}

fail() {
  printf '[FAIL] %s: exit=%s output=%s\n' "$1" "$status" "$output" >&2
  exit 1
}

run_lint bad.go
if (( status != 1 )) || [[ $output != *bad.go* ]]; then fail "misformatted Go file"; fi
printf '[PASS] misformatted Go file exits 1 and names bad.go\n'

run_lint
if (( status != 1 )) || [[ $output != *bad.go* ]]; then fail "whole project with misformatted Go file"; fi
printf '[PASS] whole-project lint exits 1 and names bad.go\n'

run_lint good.go
if (( status != 0 )); then fail "formatted Go file path"; fi
printf '[PASS] formatted Go file path exits 0\n'

# Only docs/good.md may reach mdsmith when it is the sole path argument.
rm -f "$stub_dir/args"
run_lint docs/good.md
if (( status != 0 )) || [[ $(cat "$stub_dir/args") != "$mdsmith check docs/good.md" ]]; then
  fail "path argument lints only that path"
fi
printf '[PASS] path argument lints only docs/good.md\n'

run_lint --bogus
if (( status != 2 )); then fail "unknown option"; fi
printf '[PASS] unknown option exits 2\n'

run_lint missing.md
if (( status != 2 )); then fail "missing path"; fi
printf '[PASS] missing path exits 2\n'

run_tasksdir_scenario

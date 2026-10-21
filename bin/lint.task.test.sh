#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"
testlib_offline_modules

cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_dir=$PWD
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

# The fixture is a scratch copy of the module and task tree, so the validate
# half runs against real contracts and real dependencies. Only the config and
# the task folders are copied from .taskfactory, never its worktrees.
project="$fixture/project"
mkdir -p "$project/.taskfactory" "$project/bin"
cp "$repo_dir/bin/lint" "$project/bin/lint"
cp -R "$repo_dir/cmd" "$repo_dir/internal" "$repo_dir/skills" \
  "$repo_dir/go.mod" "$repo_dir/go.sum" "$repo_dir/.mdsmith.yml" "$project/"
if [[ -d $repo_dir/.taskfactory/tasks ]]; then
  cp -R "$repo_dir/.taskfactory/config.toml" "$repo_dir/.taskfactory/tasks" "$project/.taskfactory/"
fi

# The first case runs the real mdsmith and the real `go run ./cmd/taskfactory
# validate`, the end-to-end check for both tools. The other cases test what
# bin/lint does with each half's result. They stub mdsmith (MD_STUB_EXIT sets
# its exit status) and run validate through a binary built once from the same
# sources, started in the background so it is ready when the second case needs it.
stubs="$fixture/stubs"
mkdir -p "$stubs"
real_go=$(command -v go)
cat > "$stubs/go" <<STUB
#!/usr/bin/env bash
if [[ -n "\${TASK_REAL:-}" ]]; then
  # -trimpath lets Go reuse the compiled packages of the real checkout.
  case "\$*" in run\ ./cmd/taskfactory\ *) GOFLAGS="-trimpath -buildvcs=false" exec "$real_go" "\$@" ;; esac
  exec "$real_go" "\$@"
fi
case "\$*" in
  run\ github.com/jeduden/mdsmith/*) exit "\${MD_STUB_EXIT:-0}" ;;
  run\ ./cmd/taskfactory\ *) shift 2; exec "$fixture/taskfactory" "\$@" ;;
  *) exec "$real_go" "\$@" ;;
esac
STUB
chmod +x "$stubs/go"
printf '[PASS] fixture project is ready\n'
(cd "$project" && "$real_go" build -trimpath -buildvcs=false -o "$fixture/taskfactory" ./cmd/taskfactory) &
build_pid=$!
# The validator needs a Git working tree; the fixture only needs the directory.
git -C "$project" init -q

# A self-contained ready-contract fixture, so this test never depends on a live
# task staying in .taskfactory/tasks/ready. Its one dependency, TF-003, is archived.
task_rel=.taskfactory/tasks/ready/TF-900-lint-fixture.md
pristine="$fixture/pristine.md"
cat > "$pristine" <<'TASK'
# TF-900: Lint fixture

## Goal

Exercise the combined Markdown lint and contract validation.

## Dependencies

- TF-003

## Scope

A fixture task used only by bin/lint.task.test.sh.

## Constraints

Do not use this task for any real work.

## Success criteria

### C1: The fixture check passes

Check: true

## Verification

Run the check and report the exit code.
TASK
target="$project/$task_rel"

# misformat: a second blank line after the title breaks no-multiple-blanks
# (MDS008) and leaves the contract intact.
misformat() {
  { head -n 1 "$pristine"; printf '\n\n'; tail -n +2 "$pristine"; } > "$target"
}

# break_contract: a dependency on a task that does not exist fails validation
# and leaves the Markdown intact.
break_contract() {
  sed 's/^- TF-003$/- TF-999/' "$pristine" > "$target"
}

run_lint() {  # run_lint ARGS...: sets status and output
  status=0
  output=$(cd "$project" && PATH="$stubs:$PATH" "$project/bin/lint" "$@" 2>&1) || status=$?
}

fail() {
  printf '[FAIL] %s: exit=%s output=%s\n' "$1" "$status" "$output" >&2
  exit 1
}

# Lint half fails: exit 1, the file is named as a Markdown failure, and the
# validate half still ran and passed.
misformat
TASK_REAL=1 run_lint "$task_rel"
if (( status != 1 )) || [[ $output != *"FAIL markdown lint: $task_rel"* ]] \
  || [[ $output == *"FAIL task validation"* ]] \
  || [[ $output != *"1 task file(s) valid"* ]]; then
  fail "misformatted task file (lint-fail)"
fi
printf '[PASS] misformatted task fails the lint half and passes validation\n'

# Validate half fails: exit 1, the file is named as a validation failure, and
# the Markdown lint still ran and passed.
wait "$build_pid"
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
MD_STUB_EXIT=1 run_lint "$task_rel"
if (( status != 1 )) || [[ $output != *"FAIL markdown lint: $task_rel"* ]] \
  || [[ $output != *"FAIL task validation: $task_rel"* ]]; then
  fail "both halves broken"
fi
printf '[PASS] both halves broken names both failures\n'

# Archive: .taskfactory/tasks/archive is not validated. The copy below has a broken contract
# but clean Markdown, so a validate run would fail and exit 1.
archive_rel=.taskfactory/tasks/archive/TF-990-archived-copy.md
sed -e 's/TF-900/TF-990/g' -e 's/^- TF-003$/- TF-999/' "$pristine" \
  > "$project/$archive_rel"
run_lint "$archive_rel"
if (( status != 0 )) || [[ $output == *"task validation"* ]]; then
  fail "archived task file is validated"
fi
printf '[PASS] archived task file is linted but not validated\n'

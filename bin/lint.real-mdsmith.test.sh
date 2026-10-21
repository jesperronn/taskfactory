#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=bin/testlib.sh
source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"
testlib_offline_modules

# End-to-end tests for bin/lint with the real pinned mdsmith, gofmt and the
# real linter for Bash. They are the real-tool cases for bin/lint, so each real
# run covers as many assertions as one scratch project allows: three runs in all.
cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_dir=$PWD
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT

project="$fixture/project"
mkdir -p "$project/bin" "$project/docs"
cp "$repo_dir/bin/lint" "$project/bin/lint"
cp "$repo_dir/.mdsmith.yml" "$project/.mdsmith.yml"

# Claimed task worktrees live under .taskfactory/ and are full repository
# copies. A whole-project run must ignore that tree, but not be too broad. The
# bad copies stay in place for every run below.
wt="$project/.taskfactory/worktrees/TF-999"
mkdir -p "$wt/docs" "$wt/bin"
printf '# Title\n\n\nBad spacing.\n' > "$wt/docs/bad.md"
printf 'package x\nfunc  f( ){}\n' > "$wt/bad.go"
printf "%s\n" "#!/usr/bin/env bash" "echo \$1" > "$wt/bin/bad.sh"

# Run 1, whole project: a misformatted Markdown file (two blank lines between
# blocks violate no-multiple-blanks, MDS008), a misformatted Go file and a Bash
# file shellcheck rejects. All must fail and be named; the worktree copies must
# not be.
printf '# Title\n\n\nBad spacing.\n' > "$project/docs/bad.md"
printf 'package x\nfunc  f( ){}\n' > "$project/bad.go"
printf "%s\n" "#!/usr/bin/env bash" "echo \$1" > "$project/bin/bad.sh"
bad_status=0
bad_output=$("$project/bin/lint" 2>&1) || bad_status=$?
if (( bad_status != 1 )) || [[ $bad_output != *docs/bad.md* ]]; then
  printf '[FAIL] misformatted Markdown: exit=%s output=%s\n' "$bad_status" "$bad_output" >&2
  exit 1
fi
printf '[PASS] misformatted Markdown exits %s and names docs/bad.md\n' "$bad_status"
if [[ $bad_output != *"bad.go"* ]] || [[ $bad_output != *"bin/bad.sh"* ]] ||
  [[ $bad_output == *.taskfactory* ]]; then
  printf '[FAIL] bad files outside .taskfactory: exit=%s output=%s\n' "$bad_status" "$bad_output" >&2
  exit 1
fi
printf '[PASS] bad files outside .taskfactory still fail and are named\n'

# Run 2: --autofix on the Markdown file repairs it.
fix_status=0
fix_output=$(cd "$project" && bin/lint --autofix docs/bad.md 2>&1) || fix_status=$?
if (( fix_status != 0 )); then
  printf '[FAIL] --autofix on misformatted Markdown: exit=%s output=%s\n' "$fix_status" "$fix_output" >&2
  exit 1
fi
printf '[PASS] --autofix repairs misformatted Markdown and exits 0\n'

# Run 3, whole project: only the repaired Markdown and the ignored worktree
# copies remain, so the run is clean.
rm "$project/bad.go" "$project/bin/bad.sh"
good_status=0
good_output=$("$project/bin/lint" 2>&1) || good_status=$?
if (( good_status != 0 )); then
  printf '[FAIL] formatted Markdown: exit=%s output=%s\n' "$good_status" "$good_output" >&2
  exit 1
fi
printf '[PASS] formatted Markdown exits 0\n'
printf '[PASS] whole-project lint ignores .taskfactory/worktrees and exits 0\n'

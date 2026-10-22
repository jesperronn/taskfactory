#!/usr/bin/env bash
# Usage: TF-039-run.sh BINARY A|B 1|2|3
# Runs one trial of the TF-039 experiment in a scratch repository and prints
# one TSV line: condition, type, work exit, verify result, seconds, committed.
# Condition A copies docs/worker-instructions.md from main; B uses this branch.
set -uo pipefail
bin=$1 cond=$2 type=$3
repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
fx=$repo/docs/experiments/TF-039-fixtures
case $type in 1) id=TF-901 ;; 2) id=TF-902 ;; 3) id=TF-903 ;; *) exit 2 ;; esac
dir=$(mktemp -d)
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null GOTOOLCHAIN=local
cd "$dir" || exit 1
git init -q -b main
git config commit.gpgsign false
git config user.name Trial
git config user.email trial@example.invalid
"$bin" init >/dev/null
mkdir -p docs
if [[ $cond == A ]]; then
  git -C "$repo" show main:docs/worker-instructions.md > docs/worker-instructions.md
else
  cp "$repo/docs/worker-instructions.md" docs/worker-instructions.md
fi
printf 'module trial\n\ngo 1.21\n' > go.mod
case $type in
1)
  printf 'package main\n\nfunc Double(n int) int { return n + 2 }\n\nfunc main() {}\n' > math.go
  printf 'package main\n\nimport "testing"\n\nfunc TestDouble(t *testing.T) {\n\tif Double(5) != 10 {\n\t\tt.Fatal("Double(5) != 10")\n\t}\n}\n' > math_test.go ;;
2)
  printf 'package main\n\nfunc main() {}\n' > main.go
  printf 'package main\n\nimport "testing"\n\nfunc TestClamp(t *testing.T) {\n\tif Clamp(5, 0, 3) != 3 || Clamp(-1, 0, 3) != 0 || Clamp(2, 0, 3) != 2 {\n\t\tt.Fatal("Clamp wrong")\n\t}\n}\n' > clamp_test.go ;;
3)
  printf 'package main\n\nimport "fmt"\n\nfunc main() {\nfmt.Printf("total: %%d\\n", 3, 4)\n  }\n' > report.go
  printf 'package main\n\nimport "testing"\n\nfunc TestMain2(t *testing.T) {}\n' > report_test.go ;;
esac
cp "$fx/t$type-$cond.task" ".taskfactory/tasks/inbox/$id-trial.md"
git add -A && git commit -q -m fixture
"$bin" promote "$id" >/dev/null || exit 1
"$bin" claim "$id" --owner trial >/dev/null || exit 1
start=$SECONDS
"$bin" work "$id" --adapter pi --model Ornith-1.5-35B-A3B-MLX-4bit --timeout 8m \
  > "$dir/work.out" 2>&1
work=$?
secs=$((SECONDS - start))
if "$bin" verify "$id" > "$dir/verify.out" 2>&1; then verdict=PASS; else
  verdict="FAIL: $(grep -m1 -iE 'FAIL|exit' "$dir/verify.out" | cut -c1-60)"
fi
committed=no
if [[ $(git -C ".taskfactory/worktrees/$id" rev-list --count main..HEAD 2>/dev/null) -gt 0 ]]; then
  committed=yes
fi
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$cond" "$type" "$work" "$verdict" "$secs" "$committed" "$dir"

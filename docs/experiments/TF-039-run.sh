#!/usr/bin/env bash
# Usage: [MODEL=id] TF-039-run.sh BINARY A|B 1|2|3|4|5
# MODEL defaults to the 35B model; Part 3 sets the 9B model.
# Runs one trial of the TF-039 experiment in a scratch repository and prints
# one TSV line: condition, type, work exit, verify result, seconds, committed.
# Condition A copies docs/worker-instructions.md from main; B uses this branch.
set -uo pipefail
bin=$1 cond=$2 type=$3
repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
fx=$repo/docs/experiments/TF-039-fixtures
case $type in 1) id=TF-901 ;; 2) id=TF-902 ;; 3) id=TF-903 ;; 4) id=TF-904 ;; 5) id=TF-905 ;; *) exit 2 ;; esac
model=${MODEL:-Ornith-1.5-35B-A3B-MLX-4bit}
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
4)
  printf 'package main\n\nimport "strconv"\n\nfunc Parse(s string) (int, error) { return strconv.Atoi(s) }\n' > util.go
  printf 'package main\n\nfunc LoadLimit(s string) int {\n\tn, _ := Parse(s)\n\treturn n\n}\n' > config.go
  printf 'package main\n\nfunc Run(args []string) int {\n\tif len(args) == 0 {\n\t\treturn 0\n\t}\n\tn, _ := Parse(args[0])\n\treturn n * 2\n}\n' > cmd.go
  printf 'package main\n\nfunc main() {}\n' > main.go
  printf 'package main\n\nimport "testing"\n\nfunc TestParse(t *testing.T) {\n\tif n, err := Parse("12"); err != nil || n != 12 {\n\t\tt.Fatal("Parse(12)")\n\t}\n}\n' > util_test.go
  printf 'package main\n\nimport "testing"\n\nfunc TestLoadLimit(t *testing.T) {\n\tif LoadLimit("5") != 7 {\n\t\tt.Fatal("LoadLimit(5)")\n\t}\n}\n' > config_test.go ;;
5)
  printf 'package main\n\nfunc Median(xs []float64) float64 {\n\tpanic("todo")\n}\n' > stats.go
  printf 'package main\n\nimport "fmt"\n\nfunc Summary(xs []float64) string {\n\treturn fmt.Sprintf("n=%%d", len(xs))\n}\n' > report.go
  printf 'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println(Summary([]float64{1, 2, 3, 4}))\n}\n' > main.go
  printf 'package main\n\nimport "testing"\n\nfunc TestMedianOdd(t *testing.T) {\n\tif Median([]float64{3, 1, 2}) != 2 {\n\t\tt.Fatal("odd")\n\t}\n}\n\nfunc TestMedianEven(t *testing.T) {\n\tif Median([]float64{1, 2, 3, 4}) != 2.0 {\n\t\tt.Fatal("even")\n\t}\n}\n' > stats_test.go ;;
esac
cp "$fx/t$type-$cond.task" "tasks/inbox/$id-trial.md"
git add -A && git commit -q -m fixture
"$bin" promote "$id" >/dev/null || exit 1
"$bin" claim "$id" --owner trial >/dev/null || exit 1
start=$SECONDS
"$bin" work "$id" --adapter pi --model "$model" --timeout 8m \
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

# shellcheck shell=bash
# bin/testlib.sh: shared helpers for bin/*.test.sh.
#
# Source this file from a test, then call testlib_verbose_wrap before the
# test does any work:
#
#   source "$(dirname "${BASH_SOURCE[0]}")/testlib.sh"
#   testlib_verbose_wrap "${BASH_SOURCE[0]}" "$@"
#
# Without --verbose the wrapper does nothing. With --verbose it reruns the
# same test script as a child, strips --verbose from its arguments, and pipes
# the child's output through a timer. Each [PASS] or [FAIL] line gets a
# [TIME] line with the seconds since the previous case line (or the start of
# the test). That interval includes the setup for the case that just finished,
# so it is the case's cost. Output that has no case line counts toward the
# next case. The last [TIME] line gives the whole-file total.
#
# The wrapper exits with the child's status, so the test's exit code is
# unchanged.

testlib_verbose_wrap() {
  local testlib_script=$1 testlib_arg testlib_verbose=0
  shift
  local testlib_args=()
  for testlib_arg in "$@"; do
    if [[ "${testlib_arg}" == --verbose ]]; then
      testlib_verbose=1
    else
      testlib_args+=("${testlib_arg}")
    fi
  done
  if (( ! testlib_verbose )) || [[ -n "${TESTLIB_CHILD:-}" ]]; then
    return 0
  fi
  local testlib_status=0
  TESTLIB_CHILD=1 bash "${testlib_script}" ${testlib_args[@]+"${testlib_args[@]}"} 2>&1 | testlib_stamp || testlib_status=$?
  exit "${testlib_status}"
}

# testlib_stamp: copies stdin to stdout, adding a [TIME] line after each case.
testlib_stamp() {
  perl -MTime::HiRes=time -e '
    my $start = time;
    my $prev = $start;
    while (my $line = <STDIN>) {
      print $line;
      next unless $line =~ /\b(PASS|FAIL)\b/;
      my $now = time;
      chomp(my $label = $line);
      printf "[TIME] %7.3fs %s\n", $now - $prev, $label;
      $prev = $now;
    }
    printf "[TIME] %7.3fs total\n", time - $start;
  '
}

# testlib_install_go_stub DIR: writes DIR/go, a stand-in for `go` that makes
# `go build` instant. It writes a small script at the -o path that prints
# "taskfactory version <V>" for --version, where <V> is read from the
# -X main.version=<V> linker flag, so a test still sees the arguments the
# wrapper passed. Every other go command, and every call with
# TESTLIB_REAL_GO set, runs the real go. Put DIR first on PATH to use it. Keep
# one real build per test file by setting TESTLIB_REAL_GO=1 for that call.
testlib_install_go_stub() {
  local testlib_dir=$1 testlib_real_go
  testlib_real_go=$(command -v go)
  cat > "${testlib_dir}/go" <<STUB
#!/usr/bin/env bash
if [[ -n "\${TESTLIB_REAL_GO:-}" || "\${1:-}" != build ]]; then
  exec "${testlib_real_go}" "\$@"
fi
shift
stub_out="" stub_version=""
while (( \$# )); do
  case "\$1" in
    -o) stub_out=\$2; shift ;;
    -ldflags)
      case "\$2" in *main.version=*) stub_version=\${2##*main.version=}; stub_version=\${stub_version%% *} ;; esac
      shift ;;
  esac
  shift
done
[[ -n "\${stub_out}" ]] || exit 2
mkdir -p "\$(dirname "\${stub_out}")" || exit 1
printf '#!/usr/bin/env bash\ncase "\${1:-}" in\n  --version) echo "taskfactory version %s" ;;\n  --help) echo "taskfactory stub help" ;;\nesac\n' "\${stub_version}" > "\${stub_out}"
chmod +x "\${stub_out}"
STUB
  chmod +x "${testlib_dir}/go"
}

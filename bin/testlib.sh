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

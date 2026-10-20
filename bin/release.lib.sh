# shellcheck shell=bash
# bin/release.lib.sh: version handling and bump advice for bin/release.
#
# Source this file; it has no side effects when sourced (it defines
# functions and read-only-style variables only). It never creates tags and
# never pushes. Portable to bash 3.2 (macOS) and bash 5 (Ubuntu CI).
#
# Functions and contracts:
#
#   release_parse_version <version>
#     The only place that knows the version grammar (TF-071 extends it).
#     Accepts v<major>.<minor>.<patch> without leading zeros. On success
#     prints "<major> <minor> <patch>" and returns 0; otherwise prints
#     nothing and returns 1.
#
#   release_compare_versions <a> <b>
#     SemVer precedence, numeric per component. Prints "less", "equal" or
#     "greater" (a relative to b) and returns 0; returns 2 and prints
#     nothing when either version is invalid.
#
#   release_last_tag
#     Prints the highest tag of the current repository that parses as a
#     version, by precedence and not by date. Annotated and lightweight tags
#     are handled by name; other tags are skipped. No such tag: prints
#     nothing and returns 1.
#
#   release_bump_advice [<last-tag>]
#     Reads subjects and bodies of `git log <last-tag>..HEAD` (full history
#     when no tag is given), skipping merges and subjects starting with
#     fixup!, squash! or Revert. Prints major, minor or patch (monotone: the
#     largest qualifying commit wins; nothing qualifying gives patch).
#     Subjects not matching type(scope)?!?: text get a warning on stderr.
#     Returns 0.
#
#   release_check_version <version>
#     Returns 1 (message on stderr) for an invalid version or one not
#     greater than the last tag. Warns on stderr but returns 0 when the
#     version is a smaller bump than the advice. With no tag the version
#     only has to parse and no advice-based refusal happens.

RELEASE_VERSION_REGEX='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
RELEASE_SUBJECT_REGEX='^[a-z]+(\([^)]*\))?!?: .+'
RELEASE_BREAKING_SUBJECT_REGEX='^[a-z]+(\([^)]*\))?!: .+'

release_parse_version() {
  local v=${1-}
  if [[ "${v}" =~ ${RELEASE_VERSION_REGEX} ]]; then
    printf '%s %s %s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}"
    return 0
  fi
  return 1
}

release_compare_versions() {
  local pa pb
  pa=$(release_parse_version "${1-}") || return 2
  pb=$(release_parse_version "${2-}") || return 2
  local a1 a2 a3 b1 b2 b3
  read -r a1 a2 a3 <<EOP
${pa}
EOP
  read -r b1 b2 b3 <<EOP
${pb}
EOP
  local x y z
  for x in 1 2 3; do
    eval "y=\$a${x}; z=\$b${x}"
    if ((10#${y} < 10#${z})); then
      echo less
      return 0
    elif ((10#${y} > 10#${z})); then
      echo greater
      return 0
    fi
  done
  echo equal
}

release_last_tag() {
  local tag best="" rel
  while IFS= read -r tag; do
    release_parse_version "${tag}" >/dev/null || continue
    if [[ -z "${best}" ]]; then
      best=${tag}
      continue
    fi
    rel=$(release_compare_versions "${tag}" "${best}")
    if [[ "${rel}" == greater ]]; then
      best=${tag}
    fi
  done <<EOT
$(git tag --list)
EOT
  [[ -n "${best}" ]] || return 1
  printf '%s\n' "${best}"
}

release_bump_advice() {
  local last=${1-} range=() level=patch hash subject body
  [[ -n "${last}" ]] && range=("${last}..HEAD") || range=(HEAD)
  while IFS= read -r hash; do
    [[ -n "${hash}" ]] || continue
    subject=$(git log -1 --format=%s "${hash}")
    case "${subject}" in
      fixup!* | squash!* | Revert*) continue ;;
    esac
    body=$(git log -1 --format=%b "${hash}")
    if [[ ! "${subject}" =~ ${RELEASE_SUBJECT_REGEX} ]]; then
      printf 'warning: non-conventional subject (%s): %s\n' "${hash}" "${subject}" >&2
      continue
    fi
    if [[ "${subject}" =~ ${RELEASE_BREAKING_SUBJECT_REGEX} ]] ||
      printf '%s\n' "${body}" | grep -Eq '^BREAKING[ -]CHANGE:'; then
      level=major
    elif [[ "${subject}" == feat:* || "${subject}" == feat\(* ]]; then
      [[ "${level}" == major ]] || level=minor
    fi
  done <<EOH
$(git log --no-merges --format=%H "${range[@]}")
EOH
  printf '%s\n' "${level}"
}

# Prints the bump level of <version> relative to <last>.
_release_bump_level() {
  local p l
  p=$(release_parse_version "$1")
  l=$(release_parse_version "$2")
  local p1 p2 p3 l1 l2 l3
  read -r p1 p2 p3 <<EOP
${p}
EOP
  read -r l1 l2 l3 <<EOP
${l}
EOP
  if ((p1 > l1)); then echo major; elif ((p2 > l2)); then echo minor; else echo patch; fi
}

_release_level_rank() {
  case "$1" in major) echo 3 ;; minor) echo 2 ;; *) echo 1 ;; esac
}

release_check_version() {
  local version=${1-} last rel advice given
  if ! release_parse_version "${version}" >/dev/null; then
    printf 'error: invalid version %s (want vMAJOR.MINOR.PATCH)\n' "${version}" >&2
    return 1
  fi
  if ! last=$(release_last_tag); then
    return 0
  fi
  rel=$(release_compare_versions "${version}" "${last}")
  if [[ "${rel}" != greater ]]; then
    printf 'error: %s is not greater than last tag %s\n' "${version}" "${last}" >&2
    return 1
  fi
  advice=$(release_bump_advice "${last}")
  given=$(_release_bump_level "${version}" "${last}")
  if (($(_release_level_rank "${given}") < $(_release_level_rank "${advice}"))); then
    printf 'warning: %s is a %s bump but the commits advise %s\n' "${version}" "${given}" "${advice}" >&2
  fi
  return 0
}

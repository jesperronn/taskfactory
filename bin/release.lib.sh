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
#     The only place that knows the version grammar. Labels live in
#     RELEASE_PRERELEASE_LABELS (one place; add more labels there).
#     Accepts v<major>.<minor>.<patch>, optionally followed by -<label>.<N>
#     (today only -rc.<N>, N a positive integer; no leading zeros anywhere).
#     Prints "<major> <minor> <patch>" for a stable version and
#     "<major> <minor> <patch> <label> <N>" for a prerelease, returns 0;
#     otherwise prints nothing and returns 1.
#
#   release_is_prerelease <version>
#     Returns 0 for a valid prerelease version, 1 for a valid stable one,
#     2 for an invalid one. Prints nothing. For the GitHub prerelease flag.
#
#   release_compare_versions <a> <b>
#     SemVer precedence: numeric per component, a prerelease is lower than
#     its stable version, prereleases compare by label then numerically.
#     Prints "less", "equal" or "greater" (a relative to b) and returns 0;
#     returns 2 and prints nothing when either version is invalid.
#
#   release_last_tag / release_last_stable_tag
#     Prints the highest STABLE tag of the current repository, by precedence
#     and not by date. Prerelease and other tags are skipped. No such tag:
#     prints nothing and returns 1. (release_last_tag is the TF-070 name.)
#
#   release_last_any_tag
#     Same, but prerelease tags count too (highest by precedence).
#
#   release_next_rc <stable-base-version>
#     Prints the next rc number for a base such as v1.0.0: one more than the
#     highest existing <base>-rc.<N> tag, or 1 when there is none. Returns 2
#     when the base is not a valid stable version.
#
#   release_bump_advice [<last-tag>]
#     Reads subjects and bodies of `git log <last-tag>..HEAD` (full history
#     when no tag is given), skipping merges and subjects starting with
#     fixup!, squash! or Revert. Prints major, minor or patch (monotone: the
#     largest qualifying commit wins; nothing qualifying gives patch).
#     Subjects not matching type(scope)?!?: text get a warning on stderr.
#     Returns 0. For a prerelease pass the last STABLE tag (or nothing).
#
#   release_check_version <version>
#     Returns 1 (message on stderr) for an invalid version or one not
#     greater than the last tag of any kind by precedence (so v1.0.0 after
#     v1.0.0-rc.2 is allowed). Warns on stderr but returns 0 when the version
#     is a smaller bump than the advice; the advice and the bump level are
#     computed against the last STABLE tag, also for a prerelease. With no
#     stable tag no advice-based warning happens.

RELEASE_PRERELEASE_LABELS="rc"
_release_label_alt=${RELEASE_PRERELEASE_LABELS// /|}
RELEASE_VERSION_REGEX="^v(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)(-(${_release_label_alt})\\.([1-9][0-9]*))?\$"
RELEASE_SUBJECT_REGEX='^[a-z]+(\([^)]*\))?!?: .+'
RELEASE_BREAKING_SUBJECT_REGEX='^[a-z]+(\([^)]*\))?!: .+'

release_parse_version() {
  local v=${1-}
  if [[ "${v}" =~ ${RELEASE_VERSION_REGEX} ]]; then
    if [[ -n "${BASH_REMATCH[4]}" ]]; then
      printf '%s %s %s %s %s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}" "${BASH_REMATCH[5]}" "${BASH_REMATCH[6]}"
    else
      printf '%s %s %s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}"
    fi
    return 0
  fi
  return 1
}

release_is_prerelease() {
  local p
  p=$(release_parse_version "${1-}") || return 2
  [[ "${p}" == *[a-z]* ]]
}

release_compare_versions() {
  local pa pb
  pa=$(release_parse_version "${1-}") || return 2
  pb=$(release_parse_version "${2-}") || return 2
  local -a av bv
  read -r -a av <<<"${pa}"
  read -r -a bv <<<"${pb}"
  local i
  for i in 0 1 2; do
    if ((av[i] < bv[i])); then
      echo less
      return 0
    elif ((av[i] > bv[i])); then
      echo greater
      return 0
    fi
  done
  # Equal core: a stable version outranks any prerelease of it.
  if [[ ${#av[@]} -eq 3 && ${#bv[@]} -eq 3 ]]; then
    echo equal
  elif [[ ${#av[@]} -eq 3 ]]; then
    echo greater
  elif [[ ${#bv[@]} -eq 3 ]]; then
    echo less
  elif [[ "${av[3]}" < "${bv[3]}" ]]; then
    echo less
  elif [[ "${av[3]}" > "${bv[3]}" ]]; then
    echo greater
  elif ((av[4] < bv[4])); then
    echo less
  elif ((av[4] > bv[4])); then
    echo greater
  else
    echo equal
  fi
}

# _release_last_tag_mode <stable|any>
_release_last_tag_mode() {
  local mode=$1 tag best="" rel
  while IFS= read -r tag; do
    release_parse_version "${tag}" >/dev/null || continue
    if [[ "${mode}" == stable ]] && release_is_prerelease "${tag}"; then
      continue
    fi
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

release_last_tag() { _release_last_tag_mode stable; }
release_last_stable_tag() { _release_last_tag_mode stable; }
release_last_any_tag() { _release_last_tag_mode any; }

release_next_rc() {
  local base=${1-} tag max=0 n
  if ! release_parse_version "${base}" >/dev/null || release_is_prerelease "${base}"; then
    return 2
  fi
  while IFS= read -r tag; do
    [[ "${tag}" == "${base}"-rc.* ]] || continue
    release_parse_version "${tag}" >/dev/null || continue
    n=${tag##*-rc.}
    if ((n > max)); then
      max=${n}
    fi
  done <<EOT
$(git tag --list)
EOT
  printf '%s\n' "$((max + 1))"
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
  local -a pv lv
  read -r -a pv <<<"$(release_parse_version "$1")"
  read -r -a lv <<<"$(release_parse_version "$2")"
  if ((pv[0] > lv[0])); then
    echo major
  elif ((pv[1] > lv[1])); then
    echo minor
  else
    echo patch
  fi
}

_release_level_rank() {
  case "$1" in major) echo 3 ;; minor) echo 2 ;; *) echo 1 ;; esac
}

release_check_version() {
  local version=${1-} last stable rel advice given
  if ! release_parse_version "${version}" >/dev/null; then
    printf 'error: invalid version %s (want vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-rc.N)\n' "${version}" >&2
    return 1
  fi
  if last=$(release_last_any_tag); then
    rel=$(release_compare_versions "${version}" "${last}")
    if [[ "${rel}" != greater ]]; then
      printf 'error: %s is not greater than last tag %s\n' "${version}" "${last}" >&2
      return 1
    fi
  fi
  stable=$(release_last_stable_tag) || return 0
  advice=$(release_bump_advice "${stable}")
  given=$(_release_bump_level "${version}" "${stable}")
  if (($(_release_level_rank "${given}") < $(_release_level_rank "${advice}"))); then
    printf 'warning: %s is a %s bump but the commits advise %s\n' "${version}" "${given}" "${advice}" >&2
  fi
  return 0
}

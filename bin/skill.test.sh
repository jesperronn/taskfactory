#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture_root=$(mktemp -d)
trap 'rm -rf "${fixture_root}"' EXIT

assert_status() {
  local actual=$1
  local expected=$2
  local context=$3
  if [[ "${actual}" != "${expected}" ]]; then
    printf '[FAIL] %s: expected status=%s; actual status=%s\n' "${context}" "${expected}" "${actual}" >&2
    exit 1
  fi
  printf '[PASS] %s\n' "${context}"
}

# frontmatter_ok FILE: succeeds when FILE starts with a `---` block that has a
# non-empty `name:` and a non-empty `description:` (inline or indented below).
frontmatter_ok() {
  awk '
    NR == 1 { if ($0 != "---") exit 1; next }
    /^---$/ { closed = 1; exit }
    /^name:[[:space:]]*[^[:space:]]/ { has_name = 1 }
    /^description:/ { in_desc = 1; if ($0 ~ /^description:[[:space:]]*[^[:space:]]/) has_desc = 1; next }
    in_desc && /^[[:space:]]+[^[:space:]]/ { has_desc = 1; next }
    /^[^[:space:]]/ { in_desc = 0 }
    END { exit (closed && has_name && has_desc) ? 0 : 1 }
  ' "$1"
}

skill="${repo_root}/skills/taskfactory/SKILL.md"

frontmatter_ok "${skill}" || status=$?
assert_status "${status:-0}" 0 'SKILL.md has name and description frontmatter'

printf -- '---\nname: broken\n---\n# no description\n' > "${fixture_root}/no-description.md"
status=0
frontmatter_ok "${fixture_root}/no-description.md" || status=$?
assert_status "${status}" 1 'rejects frontmatter without description'

printf -- '---\ndescription: has no name\n---\n# no name\n' > "${fixture_root}/no-name.md"
status=0
frontmatter_ok "${fixture_root}/no-name.md" || status=$?
assert_status "${status}" 1 'rejects frontmatter without name'

printf -- '# no frontmatter\n' > "${fixture_root}/no-frontmatter.md"
status=0
frontmatter_ok "${fixture_root}/no-frontmatter.md" || status=$?
assert_status "${status}" 1 'rejects a file without frontmatter'

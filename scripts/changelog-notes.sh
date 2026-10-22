#!/usr/bin/env bash
set -euo pipefail

extract_notes() {
  awk -v ver="${1#v}" '
    /^## / {
      if (found) exit
      heading = $0
      sub(/ - [0-9][0-9][0-9][0-9]-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$/, "", heading)
      if (heading == "## [" ver "]" || heading == "## " ver) { found=1; next }
    }
    found { print }
  ' "${2:-CHANGELOG.md}"
}

if [[ ${1:-} == "--test" ]]; then
  fixture=$(mktemp)
  trap 'rm -f "$fixture"' EXIT
  for heading in '## [1.2.3]' '## 1.2.3' '## [1.2.3] - 2026-10-22' '## 1.2.3 - 2026-10-22'; do
    printf '%s\nexpected\n## [1.2.4]\nwrong\n' "$heading" > "$fixture"
    [[ $(extract_notes 1.2.3 "$fixture") == expected ]]
  done
  for heading in '## [11.2.30]' '## [1.2.30]' '## [1.2.3-rc.1]' '## [1.2.3] - today' '## [1.2.3] - 2026-1-22' '## [1.2.3] - 2026-13-22' '## [1.2.3] - 2026-10-32' '## [1.2.3] extra'; do
    printf '%s\nwrong\n' "$heading" > "$fixture"
    [[ -z $(extract_notes 1.2.3 "$fixture") ]]
  done
  exit
fi
if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <version> [changelog] | --test" >&2
  exit 2
fi
extract_notes "$@"

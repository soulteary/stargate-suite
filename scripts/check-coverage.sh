#!/usr/bin/env bash

set -euo pipefail

coverage_file="${1:-coverage.out}"
minimum="${COVERAGE_MINIMUM:-30.0}"

if ! awk -v value="$minimum" 'BEGIN { exit !(value ~ /^[0-9]+([.][0-9]+)?$/) }'; then
  echo "Invalid coverage minimum: $minimum" >&2
  exit 2
fi

if [[ ! -s "$coverage_file" ]]; then
  echo "Coverage profile is missing or empty: $coverage_file" >&2
  exit 1
fi

total=$(go tool cover -func="$coverage_file" | awk '$1 == "total:" { gsub(/%/, "", $NF); print $NF }')
if ! awk -v value="$total" 'BEGIN { exit !(value ~ /^[0-9]+([.][0-9]+)?$/) }'; then
  echo "Could not read total coverage from $coverage_file" >&2
  exit 1
fi

echo "Total coverage: ${total}% (minimum: ${minimum}%)"
if ! awk -v total="$total" -v minimum="$minimum" 'BEGIN { exit !(total + 0 >= minimum + 0) }'; then
  echo "Coverage ${total}% is below the required ${minimum}%" >&2
  exit 1
fi

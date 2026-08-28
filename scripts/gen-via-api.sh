#!/bin/bash
# Deprecated shim: compose generation is now a native CLI command and no longer
# requires starting a temporary Web server or `jq`. This wrapper forwards to
# `suite generate --canonical` so any existing caller (CI, docs) keeps working.
#
# Prefer calling the CLI directly:
#   go run ./cmd/suite generate --canonical --output build
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

BUILD_DIR="${BUILD_DIR:-build}"
# Optional first arg overrides the mode list (space-separated), matching the
# previous interface; the CLI expects a comma-separated list.
MODES="${1:-}"

echo "gen-via-api.sh is deprecated; forwarding to: suite generate --canonical" >&2

if [ -n "$MODES" ]; then
  MODES_CSV="$(echo "$MODES" | tr ' ' ',')"
  exec go run ./cmd/suite generate --canonical --output "$BUILD_DIR" --modes "$MODES_CSV"
fi

exec go run ./cmd/suite generate --canonical --output "$BUILD_DIR"

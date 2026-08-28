#!/usr/bin/env bash
set -euo pipefail

is_prerelease() {
  local version=${1#v}
  version=${version%%+*}
  [[ "$version" == *-* ]]
}

run_tests() {
  local version
  for version in 1.2.3 v1.2.3 1.2.3+build-1 v1.2.3+meta.alpha-1; do
    if is_prerelease "$version"; then
      echo "stable version classified as prerelease: $version" >&2
      return 1
    fi
  done
  for version in 1.2.3-rc.1 v1.2.3-beta 1.2.3-rc.1+build-1; do
    if ! is_prerelease "$version"; then
      echo "prerelease version classified as stable: $version" >&2
      return 1
    fi
  done
}

if [[ ${1:-} == "--test" ]]; then
  run_tests
  exit
fi
if [[ $# -ne 1 ]]; then
  echo "usage: $0 <semver> | --test" >&2
  exit 2
fi
if is_prerelease "$1"; then
  echo true
else
  echo false
fi

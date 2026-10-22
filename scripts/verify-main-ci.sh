#!/usr/bin/env bash
set -euo pipefail

matches_main() {
  jq -e --arg sha "$1" '
    .path == ".github/workflows/main.yml" and
    .head_sha == $sha and .head_branch == "main" and .event == "push" and
    .status == "completed" and .conclusion == "success"
  ' >/dev/null
}

if [[ ${1:-} == --test ]]; then
  valid='{"id":123,"path":".github/workflows/main.yml","head_sha":"exact-sha","head_branch":"main","event":"push","status":"completed","conclusion":"success"}'
  matches_main exact-sha <<< "$valid"
  for field in path head_sha head_branch event status conclusion; do
    if jq --arg field "$field" '.[$field] = "incorrect"' <<< "$valid" | matches_main exact-sha; then
      echo "Accepted incorrect $field" >&2
      exit 1
    fi
  done
  if matches_main exact-sha <<< '{}'; then
    echo 'Accepted missing run fields' >&2
    exit 1
  fi
  sandbox=$(mktemp -d)
  trap 'rm -rf "$sandbox"' EXIT
  cat > "$sandbox/gh" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
if [[ $2 == *workflows/main.yml/runs* ]]; then
  count=0
  [[ ! -f $MOCK_COUNT ]] || count=$(cat "$MOCK_COUNT")
  count=$((count + 1))
  echo "$count" > "$MOCK_COUNT"
  if [[ $MOCK_MODE != empty && $count -gt 1 ]]; then echo 123; fi
else
  printf '%s\n' "$MOCK_RUN"
fi
MOCK
  printf '#!/usr/bin/env bash\nexit 0\n' > "$sandbox/sleep"
  chmod +x "$sandbox/gh" "$sandbox/sleep"
  test_sha=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  export MOCK_COUNT="$sandbox/count" MOCK_MODE=delayed
  export MOCK_RUN
  MOCK_RUN=$(jq --arg sha "$test_sha" '.head_sha=$sha' <<< "$valid")
  PATH="$sandbox:$PATH" GITHUB_REPOSITORY=test/repo "$0" "$test_sha"
  [[ $(cat "$MOCK_COUNT") == 2 ]]
  for MOCK_MODE in empty wrong; do
    export MOCK_MODE
    rm -f "$MOCK_COUNT"
    MOCK_RUN=$(jq '.conclusion="failure"' <<< "$MOCK_RUN")
    if PATH="$sandbox:$PATH" GITHUB_REPOSITORY=test/repo "$0" "$test_sha" >/dev/null 2>&1; then
      echo "Accepted $MOCK_MODE CI results" >&2
      exit 1
    fi
    [[ $(cat "$MOCK_COUNT") == 6 ]]
  done
  exit
fi
if [[ $# -ne 1 || ! $1 =~ ^[0-9a-f]{40}$ ]]; then
  echo "usage: $0 <commit-sha> | --test" >&2
  exit 2
fi
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY must be set}"
sha=$1
# List queries can briefly disagree with direct run records. Never trust a
# server-side success filter: require the complete predicate on the direct run.
for attempt in 1 2 3 4 5 6; do
  if candidates=$(gh api "repos/$GITHUB_REPOSITORY/actions/workflows/main.yml/runs?head_sha=$sha&branch=main&event=push&per_page=100" \
    --jq '.workflow_runs[].id'); then
    while IFS= read -r run_id; do
      [[ $run_id =~ ^[0-9]+$ ]] || continue
      if run=$(gh api "repos/$GITHUB_REPOSITORY/actions/runs/$run_id") && matches_main "$sha" <<< "$run"; then
        echo "Verified successful main push CI run $run_id for $sha."
        exit 0
      fi
    done <<< "$candidates"
  fi
  if [[ $attempt -lt 6 ]]; then
    echo "No directly verified main CI run yet (attempt $attempt/6); retrying in 10 seconds." >&2
    sleep 10
  fi
done
echo "Commit $sha has no directly verified successful Main push CI run; refusing to release." >&2
exit 1

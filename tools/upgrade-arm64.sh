#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
WORKSPACE_ROOT=$(CDPATH= cd -- "$REPO_ROOT/.." && pwd)
CONFIG="$WORKSPACE_ROOT/workspace.json"
# shellcheck source=tools/lib/retry.sh
source "$SCRIPT_DIR/lib/retry.sh"

usage() {
  cat <<'USAGE'
Usage: tools/upgrade-arm64.sh [--source REF] [--merge]

Fetches the configured official source with retries and verifies its VERSION.
With --merge, starts a normal no-commit merge. Conflict resolution remains
manual because this repository intentionally keeps custom deletions and local
features; the script never guesses through semantic conflicts.
USAGE
}

source_ref='upstream/main'
merge=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --source) [ "$#" -ge 2 ] || { echo 'missing value for --source' >&2; exit 2; }; source_ref=$2; shift 2 ;;
    --merge) merge=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

command -v git >/dev/null 2>&1 || { echo 'git is required' >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo 'jq is required' >&2; exit 1; }
[ -f "$CONFIG" ] || { echo "workspace config not found: $CONFIG" >&2; exit 1; }

branch=$(git -C "$REPO_ROOT" branch --show-current)
[ "$branch" = main ] || { echo "upgrade must run on main, found: $branch" >&2; exit 1; }
[ -z "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=normal)" ] || {
  echo 'working tree must be clean before fetching or merging' >&2
  exit 1
}

official_remote=$(jq -er '.versionVerification.officialRemote' "$CONFIG")
actual_upstream=$(git -C "$REPO_ROOT" remote get-url upstream)
[ "$actual_upstream" = "$official_remote" ] || {
  echo "upstream URL mismatch: $actual_upstream" >&2
  exit 1
}
expected_version=$(jq -er '.versionVerification.expectedTagVersion' "$CONFIG")

retry_with_backoff "fetch official source" \
  git -C "$REPO_ROOT" fetch --tags upstream main

source_commit=$(git -C "$REPO_ROOT" rev-parse "$source_ref")
source_version=$(git -C "$REPO_ROOT" show "$source_ref:backend/cmd/server/VERSION" | tr -d '\r\n')
[ "$source_version" = "$expected_version" ] || {
  echo "official source VERSION $source_version does not match expected $expected_version" >&2
  exit 1
}
printf 'source=%s\ncommit=%s\nversion=%s\n' "$source_ref" "$source_commit" "$source_version"

if [ "$merge" -eq 1 ]; then
  if ! git -C "$REPO_ROOT" merge --no-commit --no-ff "$source_ref"; then
    echo 'merge stopped; resolve these conflicts before committing:' >&2
    git -C "$REPO_ROOT" diff --name-only --diff-filter=U >&2 || true
    exit 1
  fi
  printf 'merge_started=true\nresolve conflicts, run checks, commit, then use tools/release-arm64.sh\n'
else
  echo 'fetch-only: add --merge to start the controlled merge'
fi

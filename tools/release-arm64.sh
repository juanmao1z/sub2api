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
Usage: tools/release-arm64.sh [--tag IMAGE[:TAG]] [--release RELEASE] [--apply]

Runs the repeatable post-merge release path: verify main and origin, build and
inspect the linux/arm64 image, push origin/main with retries, run a deployment
plan, and optionally apply the deployment. Without --apply production is not
written.
USAGE
}

tag=''
release=''
apply=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --tag) [ "$#" -ge 2 ] || { echo 'missing value for --tag' >&2; exit 2; }; tag=$2; shift 2 ;;
    --release) [ "$#" -ge 2 ] || { echo 'missing value for --release' >&2; exit 2; }; release=$2; shift 2 ;;
    --apply) apply=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

command -v git >/dev/null 2>&1 || { echo 'git is required' >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo 'jq is required' >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo 'docker is required' >&2; exit 1; }
[ -f "$CONFIG" ] || { echo "workspace config not found: $CONFIG" >&2; exit 1; }

branch=$(git -C "$REPO_ROOT" branch --show-current)
[ "$branch" = main ] || { echo "release must run on main, found: $branch" >&2; exit 1; }
[ -z "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=normal)" ] || {
  echo 'working tree must be clean before release' >&2
  exit 1
}
origin_url=$(git -C "$REPO_ROOT" remote get-url origin)
expected_origin=$(jq -er '.projects[] | select(.id == "sub2api-custom") | .remote' "$CONFIG")
[ "$origin_url" = "$expected_origin" ] || { echo "origin URL mismatch: $origin_url" >&2; exit 1; }

target_platform=$(jq -er '.target.platform' "$CONFIG")
[ "$target_platform" = linux/arm64 ] || { echo "unsupported target: $target_platform" >&2; exit 1; }
expected_version=$(jq -er '.versionVerification.expectedTagVersion' "$CONFIG")
verified_source_version=$(jq -er '.versionVerification.verifiedSourceVersionFile' "$CONFIG")
release_verified=$(jq -r '.versionVerification.releaseVersionVerified' "$CONFIG")
source_verified=$(jq -r '.versionVerification.sourceVerified' "$CONFIG")
[ "$release_verified" = true ] && [ "$source_verified" = true ] || {
  echo 'workspace version verification is not enabled' >&2
  exit 3
}
version=$(tr -d '\r\n' < "$REPO_ROOT/backend/cmd/server/VERSION")
[ "$version" = "$expected_version" ] && [ "$version" = "$verified_source_version" ] || {
  echo "source VERSION $version does not match verified VERSION $verified_source_version" >&2
  exit 3
}

retry_with_backoff "fetch origin main" git -C "$REPO_ROOT" fetch origin main
read -r behind ahead < <(git -C "$REPO_ROOT" rev-list --left-right --count origin/main...HEAD)
[ "$behind" = 0 ] || { echo "local main is behind origin/main by $behind commit(s)" >&2; exit 1; }

if [ -z "$tag" ]; then
  tag="sub2api-custom:preflight-v$version-arm64"
fi
if [ -z "$release" ]; then
  release="${version}-arm64-$(date -u +%Y%m%d-%H%M%S)"
fi

printf 'commit=%s\nversion=%s\ntag=%s\nrelease=%s\n' \
  "$(git -C "$REPO_ROOT" rev-parse HEAD)" "$version" "$tag" "$release"

build_retry_env=(RETRY_ATTEMPTS="${RETRY_ATTEMPTS:-4}" RETRY_DELAY_SECONDS="${RETRY_DELAY_SECONDS:-5}" RETRY_MAX_DELAY_SECONDS="${RETRY_MAX_DELAY_SECONDS:-30}")
env "${build_retry_env[@]}" bash "$SCRIPT_DIR/build-arm64.sh" --tag "$tag"

docker image inspect "$tag" --format '{{.Os}}/{{.Architecture}}' | grep -Fx 'linux/arm64' >/dev/null || {
  echo "image $tag is not linux/arm64" >&2
  exit 1
}

retry_with_backoff "push origin/main" git -C "$REPO_ROOT" push origin main
retry_with_backoff "verify origin/main" git -C "$REPO_ROOT" fetch origin main
[ "$(git -C "$REPO_ROOT" rev-parse HEAD)" = "$(git -C "$REPO_ROOT" rev-parse origin/main)" ] || {
  echo 'origin/main does not match the released commit' >&2
  exit 1
}

bash "$SCRIPT_DIR/deploy-arm64.sh" --tag "$tag" --release "$release"
if [ "$apply" -eq 1 ]; then
  bash "$SCRIPT_DIR/deploy-arm64.sh" --tag "$tag" --release "$release" --apply
else
  echo 'plan-only: add --apply after reviewing the plan'
fi

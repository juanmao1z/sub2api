#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
WORKSPACE_ROOT=$(CDPATH= cd -- "$REPO_ROOT/.." && pwd)
CONFIG="$WORKSPACE_ROOT/workspace.json"

usage() {
  cat <<'USAGE'
Usage: tools/build-arm64.sh [--tag IMAGE[:TAG]] [--builder NAME] [--print]

Builds the configured project with the active Docker context and Buildx builder.
HTTP_PROXY, HTTPS_PROXY, and NO_PROXY are passed only when present in the
current environment; the script does not select a proxy or Docker endpoint.
USAGE
}

tag=''
builder="${BUILDX_BUILDER:-}"
print_only=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --tag) [ "$#" -ge 2 ] || { echo 'missing value for --tag' >&2; exit 2; }; tag=$2; shift 2 ;;
    --builder) [ "$#" -ge 2 ] || { echo 'missing value for --builder' >&2; exit 2; }; builder=$2; shift 2 ;;
    --print) print_only=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

command -v docker >/dev/null 2>&1 || { echo 'docker is required' >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo 'jq is required' >&2; exit 1; }
[ -f "$CONFIG" ] || { echo "workspace config not found: $CONFIG" >&2; exit 1; }

target_platform=$(jq -er '.target.platform' "$CONFIG")
default_tag=$(jq -er '.projects[] | select(.id == "sub2api-custom") | .build.defaultTag' "$CONFIG")
[ -n "$tag" ] || tag=$default_tag
[ "$target_platform" = 'linux/arm64' ] || { echo "unsupported target platform: $target_platform" >&2; exit 1; }

context=$(docker context show 2>/dev/null) || { echo 'cannot read the active Docker context' >&2; exit 1; }
[ -n "$context" ] || { echo 'active Docker context is empty' >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo 'Docker daemon is unavailable for the active context' >&2; exit 1; }

inspect_args=(buildx inspect --bootstrap)
[ -n "$builder" ] && inspect_args+=(--builder "$builder")
builder_info=$(docker "${inspect_args[@]}" 2>&1) || {
  echo "cannot inspect the active Buildx builder${builder:+ '$builder'}" >&2
  printf '%s\n' "$builder_info" >&2
  exit 1
}
if [ -z "$builder" ]; then
  builder=$(printf '%s\n' "$builder_info" | sed -n 's/^Name:[[:space:]]*//p' | head -n 1)
fi
platforms=$(printf '%s\n' "$builder_info" | sed -n 's/^[[:space:]]*Platforms:[[:space:]]*//p' | head -n 1)
platforms_csv=$(printf '%s' "$platforms" | tr -d '[:space:]')
case ",$platforms_csv," in
  *,"$target_platform",*) : ;;
  *) echo "active Buildx builder does not advertise $target_platform: ${platforms:-unknown}" >&2; exit 1 ;;
esac

version=$(tr -d '\r\n' < "$REPO_ROOT/backend/cmd/server/VERSION")
commit=$(git -C "$REPO_ROOT" rev-parse HEAD)
if [ -n "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=normal)" ]; then
  commit="$commit-dirty"
fi
date=$(date -u +%Y-%m-%dT%H:%M:%SZ)

args=(buildx bake)
[ -n "$builder" ] && args+=(--builder "$builder")
args+=(--file "$REPO_ROOT/docker-bake.hcl"
  --set "app.platform=$target_platform"
  --set "app.tags=$tag"
  --set "app.args.VERSION=$version"
  --set "app.args.COMMIT=$commit"
  --set "app.args.DATE=$date"
  --set "app.labels.org.opencontainers.image.version=$version"
  --set "app.labels.org.opencontainers.image.revision=$commit"
  --set "app.labels.org.opencontainers.image.created=$date")

for proxy_name in HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy GOPROXY GOSUMDB; do
  proxy_value=${!proxy_name-}
  [ -n "$proxy_value" ] && args+=(--set "app.args.$proxy_name=$proxy_value")
done
[ "$print_only" -eq 1 ] && args+=(--print)

printf 'context=%s\nbuilder=%s\ntarget=%s\ntag=%s\n' "$context" "${builder:-active}" "$target_platform" "$tag"
(cd "$REPO_ROOT" && docker "${args[@]}")

if [ "$print_only" -eq 0 ]; then
  image_arch=$(docker image inspect "$tag" --format '{{.Architecture}}/{{.Os}}')
  [ "$image_arch" = 'arm64/linux' ] || {
    echo "built image has architecture $image_arch, expected arm64/linux" >&2
    exit 1
  }
  printf 'image=%s\narchitecture=%s\n' "$tag" "$image_arch"
fi

#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
version=$(tr -d '\r\n' < "$REPO_ROOT/backend/cmd/server/VERSION")

tag="sub2api-custom:preflight-v${version}-arm64"
release="${version}-arm64-$(date -u +%Y%m%d-%H%M%S)"
apply=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --tag) [ "$#" -ge 2 ] || { echo 'missing value for --tag' >&2; exit 2; }; tag=$2; shift 2 ;;
    --release) [ "$#" -ge 2 ] || { echo 'missing value for --release' >&2; exit 2; }; release=$2; shift 2 ;;
    --apply) apply=1; shift ;;
    -h|--help)
      exec bash "$SCRIPT_DIR/sub2api-release.sh" --help
      ;;
    *) echo "unknown argument: $1" >&2; echo 'use tools/sub2api-release.sh for staged commands' >&2; exit 2 ;;
  esac
done

args=(run --tag "$tag" --release "$release")
[ "$apply" -eq 1 ] && args+=(--apply)
exec bash "$SCRIPT_DIR/sub2api-release.sh" "${args[@]}"

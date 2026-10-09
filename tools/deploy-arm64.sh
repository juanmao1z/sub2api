#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
WORKSPACE_ROOT=$(CDPATH= cd -- "$REPO_ROOT/.." && pwd)
CONFIG="$WORKSPACE_ROOT/workspace.json"
source "$SCRIPT_DIR/lib/retry.sh"
source "$SCRIPT_DIR/lib/release-checks.sh"

usage() {
  cat <<'USAGE'
Usage: tools/deploy-arm64.sh [--tag IMAGE[:TAG]] [--release RELEASE] [--upload-only | --apply]

Default: print a plan without Docker/SSH access or production writes.
--upload-only: upload, checksum-verify and load the image; do not switch services.
--apply: upload and recreate only the configured application, with health rollback.
Both writing modes require clean, committed release sources and a matching ARM64 image.
USAGE
}

tag=''
release=''
mode=plan
while [ "$#" -gt 0 ]; do
  case "$1" in
    --tag|--release)
      [ "$#" -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }
      if [ "$1" = --tag ]; then tag=$2; else release=$2; fi
      shift 2 ;;
    --apply|--upload-only)
      [ "$mode" = plan ] || { echo '--apply and --upload-only are mutually exclusive' >&2; exit 2; }
      if [ "$1" = --apply ]; then mode=apply; else mode=upload; fi
      shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

for tool in jq git; do
  command -v "$tool" >/dev/null 2>&1 || { echo "$tool is required" >&2; exit 1; }
done
[ -f "$CONFIG" ] || { echo "workspace config not found: $CONFIG" >&2; exit 1; }
target_platform=$(jq -er '.target.platform' "$CONFIG")
target_architecture=$(jq -er '.target.architecture' "$CONFIG")
project_image=$(jq -er '.projects[] | select(.id == "sub2api-custom") | .build.image' "$CONFIG")
default_tag=$(jq -er '.projects[] | select(.id == "sub2api-custom") | .build.defaultTag' "$CONFIG")
ssh_alias=$(jq -er '.production.sshAlias' "$CONFIG")
deploy_root=$(jq -er '.production.deployRoot' "$CONFIG")
image_root=$(jq -er '.production.imageRoot' "$CONFIG")
public_base_url=$(jq -er '.production.publicBaseUrl' "$CONFIG")
app_service=$(jq -er '.production.appService' "$CONFIG")
release_compose=$(jq -er '.production.releaseComposeFile' "$CONFIG")
mapfile -t compose_files < <(jq -er '.production.composeFiles[]' "$CONFIG")
mapfile -t preserved_services < <(jq -er '.production.preservedServices[]' "$CONFIG")
[ "${#compose_files[@]}" -gt 0 ] && [ "${#preserved_services[@]}" -gt 0 ] || { echo 'composeFiles and preservedServices must not be empty' >&2; exit 1; }
jq -e '.target.platform == "linux/arm64" and .target.architecture == "arm64" and
  .production.platform == .target.platform and .production.architecture == .target.architecture and
  (.production.composeFiles | all(type == "string" and length > 0)) and
  (.production.preservedServices | all(type == "string" and length > 0))' "$CONFIG" >/dev/null || { echo 'inconsistent production target configuration' >&2; exit 1; }
for value in "$ssh_alias" "$project_image" "$app_service" "${preserved_services[@]}"; do
  case "$value" in ''|-*|*[!A-Za-z0-9._:/-]*) echo "unsafe deployment identifier: $value" >&2; exit 2 ;; esac
done
for value in "$deploy_root" "$image_root"; do
  case "$value" in /*) : ;; *) echo 'deployment roots must be absolute' >&2; exit 2 ;; esac
  case "$value" in *[!A-Za-z0-9._/-]*) echo "unsafe deployment path: $value" >&2; exit 2 ;; esac
done
case "$release_compose" in ''|.*|*/*|*[!A-Za-z0-9._-]*) echo 'releaseComposeFile must be a safe filename' >&2; exit 2 ;; esac
for value in "${compose_files[@]}"; do
  case "$value" in ''|*[!A-Za-z0-9._/-]*) echo "unsafe compose path: $value" >&2; exit 2 ;; esac
  [ "$value" != "$release_compose" ] || { echo 'releaseComposeFile must be separate from composeFiles' >&2; exit 2; }
done
verify_release_source
version=$(tr -d '\r\n' < "$REPO_ROOT/backend/cmd/server/VERSION")
[ -n "$tag" ] || tag=$default_tag
[ -n "$release" ] || release="${version}-${target_architecture}-$(date -u +%Y%m%d-%H%M%S)"
case "$release" in ''|.*|*[!A-Za-z0-9._-]*) echo "unsafe release name: $release" >&2; exit 2 ;; esac
case "$tag" in ''|-*|*[!A-Za-z0-9._:/-]*) echo "unsafe image tag: $tag" >&2; exit 2 ;; esac
release_image="$project_image:$release"
stage="$image_root/$release"
printf 'mode=%s\nsource_image=%s\nrelease_image=%s\ntarget=%s\nssh=%s\napp_service=%s\nversion=%s\n' \
  "$mode" "$tag" "$release_image" "$target_platform" "$ssh_alias" "$app_service" "$version"
printf 'official_release=%s\nofficial_source_version=%s\nrelease_compose=%s/%s\npreserved_services=%s\n' \
  "$(jq -er '.versionVerification.latestVerifiedTag' "$CONFIG")" "$(jq -er '.versionVerification.verifiedSourceVersionFile' "$CONFIG")" \
  "$deploy_root" "$release_compose" "${preserved_services[*]}"
[ "$mode" != plan ] || { echo 'plan-only: use --upload-only to stage an image, or --apply to switch the application'; exit 0; }

[ -z "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=normal)" ] || { echo 'remote writes require a clean, committed working tree (resolve pending merges first)' >&2; exit 3; }
for tool in docker ssh scp curl gzip sha256sum; do
  command -v "$tool" >/dev/null 2>&1 || { echo "$tool is required for $mode" >&2; exit 1; }
done
revision=$(image_revision)
verify_release_image "$tag" "$revision"
image_id=$(docker image inspect "$tag" --format '{{.Id}}')
ssh_options=(-o BatchMode=yes -o ConnectTimeout=10 -o ServerAliveInterval=5 -o ServerAliveCountMax=3)
preserved_csv=$(IFS=,; printf '%s' "${preserved_services[*]}")

remote_preflight() {
  ssh "${ssh_options[@]}" "$ssh_alias" bash -s -- "$deploy_root" "$app_service" "$target_architecture" "$release_compose" "$preserved_csv" "${compose_files[@]}" <<'PREFLIGHT'
set -Eeuo pipefail
deploy_root=$1; app_service=$2; expected_arch=$3; release_compose=$4; preserved_csv=$5; shift 5
sudo -n true
case "$(uname -m)" in aarch64|arm64) actual_arch=arm64 ;; *) actual_arch=$(uname -m) ;; esac
[ "$actual_arch" = "$expected_arch" ] || { echo "remote architecture mismatch: $actual_arch" >&2; exit 1; }
[ "$(sudo -n docker version --format '{{.Server.Os}}/{{.Server.Arch}}')" = "linux/$expected_arch" ] || { echo 'remote Docker architecture mismatch' >&2; exit 1; }
compose=(sudo -n docker compose --project-directory "$deploy_root")
for file in "$@"; do
  case "$file" in /*) path=$file ;; *) path="$deploy_root/$file" ;; esac
  [ -r "$path" ] || { echo "missing or unreadable Compose file: $path" >&2; exit 1; }
  compose+=(-f "$path")
done
[ ! -f "$deploy_root/$release_compose" ] || compose+=(-f "$deploy_root/$release_compose")
"${compose[@]}" config --quiet
app_id=$("${compose[@]}" ps --all -q "$app_service")
[ -n "$app_id" ] || { echo 'application container not found in configured Compose project' >&2; exit 1; }
[ "$(sudo -n docker inspect "$app_id" --format '{{.State.Status}}/{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}')" = running/healthy ] || { echo 'current application is not healthy' >&2; exit 1; }
IFS=, read -r -a preserved <<< "$preserved_csv"
for service in "${preserved[@]}"; do
  [ "$(sudo -n docker inspect "$service" --format '{{.State.Status}}')" = running ] || { echo "preserved service not running: $service" >&2; exit 1; }
done
printf 'remote_preflight=passed\n'
PREFLIGHT
}
retry_with_backoff 'production preflight' remote_preflight
health_url="${public_base_url%/}/health"
retry_with_backoff 'public health preflight' curl --fail --silent --show-error --max-time 20 "$health_url" >/dev/null

archive_root="$REPO_ROOT/deploy-artifacts/$release"
archive="$archive_root/image.tar.gz"
mkdir -p "$archive_root"
docker tag "$tag" "$release_image"
docker save "$release_image" | gzip -n > "$archive.tmp"
mv "$archive.tmp" "$archive"
archive_hash=$(sha256sum "$archive" | awk '{print $1}')
remote_archive="/tmp/sub2api-$release-$archive_hash.tar.gz"
retry_with_backoff 'image archive upload' scp "${ssh_options[@]}" "$archive" "$ssh_alias:$remote_archive"
ssh "${ssh_options[@]}" "$ssh_alias" bash -s -- "$stage" "$release_image" "$target_architecture" "$remote_archive" "$archive_hash" "$image_id" "$version" "$revision" <<'UPLOAD'
set -Eeuo pipefail
stage=$1; image=$2; architecture=$3; archive=$4; hash=$5; image_id=$6; version=$7; revision=$8
printf '%s  %s\n' "$hash" "$archive" | sha256sum -c -
sudo -n mkdir -p "$stage"
sudo -n install -m 600 "$archive" "$stage/image.tar.gz"
sudo -n gzip -dc "$stage/image.tar.gz" | sudo -n docker load
[ "$(sudo -n docker image inspect "$image" --format '{{.Os}}/{{.Architecture}}')" = "linux/$architecture" ]
[ "$(sudo -n docker image inspect "$image" --format '{{.Id}}')" = "$image_id" ]
[ "$(sudo -n docker image inspect "$image" --format '{{index .Config.Labels "org.opencontainers.image.version"}}')" = "$version" ]
[ "$(sudo -n docker image inspect "$image" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')" = "$revision" ]
rm "$archive"
printf 'upload=verified\nimage=%s\nimage_id=%s\n' "$image" "$image_id"
UPLOAD
printf 'archive=%s\narchive_sha256=%s\n' "$archive" "$archive_hash"
[ "$mode" != upload ] || { echo 'upload-only: no service switch performed'; exit 0; }

remote_output=$(ssh "${ssh_options[@]}" "$ssh_alias" bash -s -- "$deploy_root" "$stage" "$release_image" "$app_service" "$release_compose" "$preserved_csv" "${compose_files[@]}" <<'REMOTE'
set -Eeuo pipefail
deploy_root=$1; stage=$2; image=$3; app_service=$4; release_compose=$5; preserved_csv=$6; shift 6
base_compose=(sudo -n docker compose --project-directory "$deploy_root")
for file in "$@"; do
  case "$file" in /*) path=$file ;; *) path="$deploy_root/$file" ;; esac
  base_compose+=(-f "$path")
done
override="$deploy_root/$release_compose"
backup_dir="$stage/rollback-$(date -u +%Y%m%d-%H%M%S)-$$"
sudo -n mkdir -m 700 "$backup_dir"
if sudo -n test -f "$override"; then
  sudo -n cp "$override" "$backup_dir/release.json"
else
  sudo -n touch "$backup_dir/override-absent"
fi
compose() {
  local args=("${base_compose[@]}")
  [ ! -f "$override" ] || args+=(-f "$override")
  "${args[@]}" "$@"
}
IFS=, read -r -a preserved <<< "$preserved_csv"
ids=()
for service in "${preserved[@]}"; do ids+=("$(sudo -n docker inspect "$service" --format '{{.Id}}')"); done
check_preserved() {
  local i
  for i in "${!preserved[@]}"; do
    [ "$(sudo -n docker inspect "${preserved[$i]}" --format '{{.Id}}')" = "${ids[$i]}" ] || return 1
    [ "$(sudo -n docker inspect "${preserved[$i]}" --format '{{.State.Status}}')" = running ] || return 1
  done
}
wait_healthy() {
  local id attempt
  for attempt in $(seq 1 45); do
    id=$(compose ps --all -q "$app_service")
    if [ -n "$id" ] && [ "$(sudo -n docker inspect "$id" --format '{{.State.Status}}/{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' 2>/dev/null || true)" = running/healthy ]; then return 0; fi
    sleep 2
  done
  return 1
}
rollback() {
  if sudo -n test -f "$backup_dir/override-absent"; then sudo -n rm -f "$override"; else sudo -n cp "$backup_dir/release.json" "$override"; fi
  compose up -d --no-deps --pull never "$app_service" && wait_healthy && check_preserved
}
old_id=$(compose ps --all -q "$app_service")
old_image=$(sudo -n docker inspect "$old_id" --format '{{.Config.Image}}')
printf 'OLD_IMAGE=%s\nBACKUP_DIR=%s\n' "$old_image" "$backup_dir"
# A JSON Compose override affects only the application image and leaves existing YAML untouched.
printf '{"services":{"%s":{"image":"%s"}}}\n' "$app_service" "$image" | sudo -n tee "$override.tmp" >/dev/null
sudo -n mv "$override.tmp" "$override"
if ! compose config --quiet || ! compose up -d --no-deps --pull never "$app_service" || ! wait_healthy || ! check_preserved; then
  rollback || { echo 'application rollback failed' >&2; exit 1; }
  echo 'rollback_health=healthy' >&2
  exit 1
fi
new_id=$(compose ps --all -q "$app_service")
[ "$(sudo -n docker inspect "$new_id" --format '{{.Config.Image}}')" = "$image" ] || { rollback; exit 1; }
printf 'SWITCHED_IMAGE=%s\nBACKUP_DIR=%s\npreserved_services=unchanged\n' "$image" "$backup_dir"
REMOTE
)
printf '%s\n' "$remote_output"
backup_dir=$(printf '%s\n' "$remote_output" | sed -n 's/^BACKUP_DIR=//p' | tail -n 1)
[ -n "$backup_dir" ] || { echo 'remote deployment returned no rollback directory' >&2; exit 1; }

if ! health_body=$(retry_with_backoff 'public health check' curl --fail --silent --show-error --max-time 20 "$health_url"); then
  echo 'public health failed; restoring the previous application image' >&2
  ssh "${ssh_options[@]}" "$ssh_alias" bash -s -- "$deploy_root" "$backup_dir" "$app_service" "$release_compose" "${compose_files[@]}" <<'ROLLBACK'
set -Eeuo pipefail
deploy_root=$1; backup_dir=$2; app_service=$3; release_compose=$4; shift 4
override="$deploy_root/$release_compose"
if sudo -n test -f "$backup_dir/override-absent"; then sudo -n rm -f "$override"; else sudo -n cp "$backup_dir/release.json" "$override"; fi
compose=(sudo -n docker compose --project-directory "$deploy_root")
for file in "$@"; do
  case "$file" in /*) path=$file ;; *) path="$deploy_root/$file" ;; esac
  compose+=(-f "$path")
done
[ ! -f "$override" ] || compose+=(-f "$override")
"${compose[@]}" up -d --no-deps --pull never "$app_service"
for attempt in $(seq 1 45); do
  id=$("${compose[@]}" ps --all -q "$app_service")
  if [ -n "$id" ] && [ "$(sudo -n docker inspect "$id" --format '{{.State.Status}}/{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' 2>/dev/null || true)" = running/healthy ]; then echo 'rollback_health=healthy'; exit 0; fi
  sleep 2
done
exit 1
ROLLBACK
  retry_with_backoff 'rollback public health' curl --fail --silent --show-error --max-time 20 "$health_url" >/dev/null
  exit 1
fi
printf 'public_health=%s\n' "$health_body"

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
Usage: tools/deploy-arm64.sh --tag IMAGE[:TAG] [--release RELEASE] [--apply]

Without --apply, prints a deployment plan only. An apply requires a verified
release version, a local linux/arm64 image, the configured SSH alias, and a
healthy public /health endpoint. Only the configured application service is
recreated; PostgreSQL, Redis, leaderboard, and their data are not touched.
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

command -v jq >/dev/null 2>&1 || { echo 'jq is required' >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo 'docker is required' >&2; exit 1; }
command -v ssh >/dev/null 2>&1 || { echo 'ssh is required' >&2; exit 1; }
command -v scp >/dev/null 2>&1 || { echo 'scp is required' >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo 'curl is required' >&2; exit 1; }
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
configured_architecture=$(jq -er '.production.architecture' "$CONFIG")
release_version_verified=$(jq -r '.versionVerification.releaseVersionVerified' "$CONFIG")
source_verified=$(jq -r '.versionVerification.sourceVerified' "$CONFIG")
version_status=$(jq -er '.versionVerification.status' "$CONFIG")
version_tag=$(jq -er '.versionVerification.latestVerifiedTag' "$CONFIG")
tag_version=$(jq -er '.versionVerification.tagVersionFile' "$CONFIG")
source_commit=$(jq -er '.versionVerification.verifiedSourceCommit' "$CONFIG")
verified_source_version=$(jq -er '.versionVerification.verifiedSourceVersionFile' "$CONFIG")
expected_tag_version=$(jq -er '.versionVerification.expectedTagVersion' "$CONFIG")
mapfile -t compose_files < <(jq -er '.production.composeFiles[]' "$CONFIG")
mapfile -t preserved_services < <(jq -er '.production.preservedServices[]' "$CONFIG")

[ "$target_platform" = 'linux/arm64' ] || { echo "unsupported target platform: $target_platform" >&2; exit 1; }
[ "$configured_architecture" = "$target_architecture" ] || { echo 'workspace target and production architecture disagree' >&2; exit 1; }
[ -n "$tag" ] || tag=$default_tag
version=$(tr -d '\r\n' < "$REPO_ROOT/backend/cmd/server/VERSION")
[ -n "$release" ] || release="${version}-${target_architecture}-$(date -u +%Y%m%d-%H%M%S)"
case "$release" in
  ''|*[!A-Za-z0-9._-]*) echo "unsafe release name: $release" >&2; exit 2 ;;
esac
release_image="$project_image:$release"
stage="$image_root/$release"

printf 'source_image=%s\nrelease_image=%s\ntarget=%s\nssh=%s\napp_service=%s\n' \
  "$tag" "$release_image" "$target_platform" "$ssh_alias" "$app_service"
printf 'preserved_services=%s\n' "${preserved_services[*]}"
printf 'version=%s\nofficial_release=%s (tag VERSION %s)\nverified_source=%s (VERSION %s)\nversion_status=%s\n' \
  "$version" "$version_tag" "$tag_version" "$source_commit" "$verified_source_version" "$version_status"

if [ "$release_version_verified" != 'true' ] || [ "$source_verified" != 'true' ] || \
  [ "$version" != "$verified_source_version" ] || [ "$verified_source_version" != "$expected_tag_version" ]; then
  cat >&2 <<BLOCKED
Production deployment is blocked: source version $version, verified official source
$source_commit version $verified_source_version, expected release version
$expected_tag_version, and verification status $version_status must all agree.
The official release tag $version_tag contains VERSION $tag_version. No production
write or image switch will be attempted until the source version gate passes.
BLOCKED
  exit 3
fi


[ "$apply" -eq 1 ] || { echo 'plan-only: add --apply after all checks pass'; exit 0; }

ssh_options=(-o BatchMode=yes -o ConnectTimeout=10 -o ServerAliveInterval=5 -o ServerAliveCountMax=3)
retry_with_backoff "production SSH preflight" ssh "${ssh_options[@]}" "$ssh_alias" true
image_arch=$(docker image inspect "$tag" --format '{{.Architecture}}/{{.Os}}')
[ "$image_arch" = "$target_architecture/linux" ] || {
  echo "local image $tag has architecture $image_arch, expected $target_architecture/linux" >&2
  exit 1
}

archive_root="$REPO_ROOT/deploy-artifacts/$release"
image_tar="$archive_root/$project_image-$release.tar"
archive="$archive_root/image.tar.gz"
mkdir -p "$archive_root"
docker tag "$tag" "$release_image"
docker save --output "$image_tar" "$release_image"
gzip -c "$image_tar" > "$archive"
archive_hash=$(sha256sum "$archive" | awk '{print $1}')
remote_archive="/tmp/sub2api-custom-$release.tar.gz"
retry_with_backoff "image archive upload" scp "${ssh_options[@]}" "$archive" "$ssh_alias:$remote_archive"

remote_output=$(ssh "${ssh_options[@]}" "$ssh_alias" bash -s -- \
  "$deploy_root" "$stage" "$release_image" "$app_service" "$target_architecture" "$remote_archive" "$archive_hash" "${compose_files[@]}" <<'REMOTE'
set -Eeuo pipefail

deploy_root=$1
stage=$2
new_image=$3
app_service=$4
expected_architecture=$5
remote_archive=$6
expected_hash=$7
shift 7
compose=(sudo docker compose)
for compose_file in "$@"; do
  case "$compose_file" in
    /*) compose_path=$compose_file ;;
    *) compose_path="$deploy_root/$compose_file" ;;
  esac
  compose+=(-f "$compose_path")
done
override="$deploy_root/docker-compose.override.yml"
backup_dir="$stage/rollback-$(date -u +%Y%m%d-%H%M%S)"

sudo mkdir -p "$stage" "$backup_dir"
sudo mv "$remote_archive" "$stage/image.tar.gz"
printf '%s  %s\n' "$expected_hash" "$stage/image.tar.gz" | sudo sha256sum -c -
sudo gzip -dc "$stage/image.tar.gz" | sudo docker load

actual_arch=$(uname -m)
case "$actual_arch" in
  aarch64|arm64) normalized_arch=arm64 ;;
  x86_64|amd64) normalized_arch=amd64 ;;
  *) normalized_arch=$actual_arch ;;
esac
[ "$normalized_arch" = "$expected_architecture" ] || {
  echo "remote server architecture $normalized_arch does not match $expected_architecture" >&2
  exit 1
}
loaded_arch=$(sudo docker image inspect "$new_image" --format '{{.Architecture}}/{{.Os}}')
[ "$loaded_arch" = "$expected_architecture/linux" ] || {
  echo "loaded image architecture $loaded_arch does not match $expected_architecture/linux" >&2
  exit 1
}
[ -f "$override" ] || { echo "missing production override: $override" >&2; exit 1; }

sudo cp "$override" "$backup_dir/docker-compose.override.yml"
sudo docker compose "${compose[@]:3}" config | sudo tee "$backup_dir/compose-config.yml" >/dev/null
old_image=$(sudo docker inspect "$app_service" --format '{{.Config.Image}}')
printf 'OLD_IMAGE=%s\nBACKUP_DIR=%s\n' "$old_image" "$backup_dir"

grep -Eq '^[[:space:]]*image:' "$override" || { echo "no image entry in $override" >&2; exit 1; }
sudo sed -i -E "0,/^[[:space:]]*image:/s#^[[:space:]]*image:.*#    image: $new_image#" "$override"

wait_healthy() {
  attempt=1
  while [ "$attempt" -le 45 ]; do
    state=$(sudo docker inspect "$app_service" --format '{{.State.Status}}' 2>/dev/null || true)
    health=$(sudo docker inspect "$app_service" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' 2>/dev/null || true)
    printf 'state=%s health=%s attempt=%s\n' "$state" "$health" "$attempt"
    [ "$state" = running ] && [ "$health" = healthy ] && return 0
    sleep 2
    attempt=$((attempt + 1))
  done
  return 1
}

rollback() {
  echo 'rolling back application compose configuration' >&2
  sudo cp "$backup_dir/docker-compose.override.yml" "$override"
  "${compose[@]}" up -d --no-deps "$app_service"
  wait_healthy
}

if ! "${compose[@]}" up -d --no-deps "$app_service"; then
  rollback || { echo 'rollback failed after compose start failure' >&2; exit 1; }
  echo 'rollback_health=healthy'
  exit 1
fi
if ! wait_healthy; then
  rollback || { echo 'rollback failed after unhealthy new container' >&2; exit 1; }
  echo 'rollback_health=healthy'
  exit 1
fi
printf 'SWITCHED_IMAGE=%s\nBACKUP_DIR=%s\n' "$new_image" "$backup_dir"
REMOTE
)
printf '%s\n' "$remote_output"
backup_dir=$(printf '%s\n' "$remote_output" | sed -n 's/^BACKUP_DIR=//p' | tail -n 1)
[ -n "$backup_dir" ] || { echo 'remote deployment did not return a rollback directory' >&2; exit 1; }

health_url="${public_base_url%/}/health"
if ! health_body=$(retry_with_backoff "public health check" curl --fail --silent --show-error --max-time 20 "$health_url"); then
  echo "public health check failed; restoring the previous application image" >&2
  ssh -o BatchMode=yes -o ConnectTimeout=10 "$ssh_alias" bash -s -- \
    "$deploy_root" "$backup_dir" "$app_service" "${compose_files[@]}" <<'ROLLBACK'
set -Eeuo pipefail
deploy_root=$1
backup_dir=$2
app_service=$3
shift 3
compose=(sudo docker compose)
for compose_file in "$@"; do
  case "$compose_file" in
    /*) compose_path=$compose_file ;;
    *) compose_path="$deploy_root/$compose_file" ;;
  esac
  compose+=(-f "$compose_path")
done
override="$deploy_root/docker-compose.override.yml"
sudo cp "$backup_dir/docker-compose.override.yml" "$override"
"${compose[@]}" up -d --no-deps "$app_service"
for attempt in $(seq 1 45); do
  state=$(sudo docker inspect "$app_service" --format '{{.State.Status}}' 2>/dev/null || true)
  health=$(sudo docker inspect "$app_service" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' 2>/dev/null || true)
  [ "$state" = running ] && [ "$health" = healthy ] && { echo 'rollback_health=healthy'; exit 0; }
  sleep 2
done
echo 'rollback_health=unhealthy' >&2
exit 1
ROLLBACK
  retry_with_backoff "rollback public health check" curl --fail --silent --show-error --max-time 20 "$health_url" >/dev/null || {
    echo 'rollback completed remotely but public health is still failing' >&2
    exit 1
  }
  echo 'rollback verified after public health failure' >&2
  exit 1
fi
printf 'public_health=%s\narchive_sha256=%s\n' "$health_body" "$archive_hash"

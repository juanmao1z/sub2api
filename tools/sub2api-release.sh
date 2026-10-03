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
Usage:
  tools/sub2api-release.sh prepare --release RELEASE [--tag IMAGE[:TAG]]
  tools/sub2api-release.sh verify --release RELEASE
  tools/sub2api-release.sh build --release RELEASE
  tools/sub2api-release.sh publish --release RELEASE
  tools/sub2api-release.sh plan --release RELEASE
  tools/sub2api-release.sh apply --release RELEASE
  tools/sub2api-release.sh status --release RELEASE
  tools/sub2api-release.sh recover --release RELEASE
  tools/sub2api-release.sh run --release RELEASE [--tag IMAGE[:TAG]] [--apply]

Stages are persisted in deploy-artifacts/<release>/release.json. `run` executes
prepare, verify, build, publish, and plan. It applies production only with
--apply. `status` is read-only. `recover` inspects an uncertain apply and never
blindly repeats a remote service switch.
USAGE
}

command_name=${1:-}
[ -n "$command_name" ] || { usage >&2; exit 2; }
shift

release=''
tag=''
apply=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --release) [ "$#" -ge 2 ] || { echo 'missing value for --release' >&2; exit 2; }; release=$2; shift 2 ;;
    --tag) [ "$#" -ge 2 ] || { echo 'missing value for --tag' >&2; exit 2; }; tag=$2; shift 2 ;;
    --apply) apply=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

command -v jq >/dev/null 2>&1 || { echo 'jq is required' >&2; exit 1; }
command -v git >/dev/null 2>&1 || { echo 'git is required' >&2; exit 1; }
case "$command_name" in
  build) command -v docker >/dev/null 2>&1 || { echo 'docker is required for build' >&2; exit 1; } ;;
  status|recover) command -v ssh >/dev/null 2>&1 || { echo 'ssh is required for status/recover' >&2; exit 1; }; command -v curl >/dev/null 2>&1 || { echo 'curl is required for status/recover' >&2; exit 1; } ;;
esac
[ -f "$CONFIG" ] || { echo "workspace config not found: $CONFIG" >&2; exit 1; }

load_config() {
  target_platform=$(jq -er '.target.platform' "$CONFIG")
  target_architecture=$(jq -er '.target.architecture' "$CONFIG")
  project_path=$(jq -er '.projects[] | select(.id == "sub2api-custom") | .path' "$CONFIG")
  project_image=$(jq -er '.projects[] | select(.id == "sub2api-custom") | .build.image' "$CONFIG")
  default_tag=$(jq -er '.projects[] | select(.id == "sub2api-custom") | .build.defaultTag' "$CONFIG")
  expected_origin=$(jq -er '.projects[] | select(.id == "sub2api-custom") | .remote' "$CONFIG")
  official_remote=$(jq -er '.versionVerification.officialRemote' "$CONFIG")
  expected_version=$(jq -er '.versionVerification.expectedTagVersion' "$CONFIG")
  verified_source_version=$(jq -er '.versionVerification.verifiedSourceVersionFile' "$CONFIG")
  release_verified=$(jq -r '.versionVerification.releaseVersionVerified' "$CONFIG")
  source_verified=$(jq -r '.versionVerification.sourceVerified' "$CONFIG")
  ssh_alias=$(jq -er '.production.sshAlias' "$CONFIG")
  deploy_root=$(jq -er '.production.deployRoot' "$CONFIG")
  image_root=$(jq -er '.production.imageRoot' "$CONFIG")
  public_base_url=$(jq -er '.production.publicBaseUrl' "$CONFIG")
  app_service=$(jq -er '.production.appService' "$CONFIG")
  configured_architecture=$(jq -er '.production.architecture' "$CONFIG")
  mapfile -t compose_files < <(jq -er '.production.composeFiles[]' "$CONFIG")
  mapfile -t preserved_services < <(jq -er '.production.preservedServices[]' "$CONFIG")
}

load_config
[ "$target_platform" = 'linux/arm64' ] || { echo "unsupported target: $target_platform" >&2; exit 1; }
[ "$target_architecture" = 'arm64' ] && [ "$configured_architecture" = 'arm64' ] || {
  echo 'workspace architecture configuration is inconsistent' >&2
  exit 1
}

if [ -n "$release" ]; then
  case "$release" in
    ''|*[!A-Za-z0-9._-]*) echo "unsafe release name: $release" >&2; exit 2 ;;
  esac
  release_dir="$REPO_ROOT/deploy-artifacts/$release"
  manifest="$release_dir/release.json"
fi

now() { date -u +%Y-%m-%dT%H:%M:%SZ; }
require_release() { [ -n "$release" ] || { echo '--release is required' >&2; exit 2; }; }
require_manifest() { require_release; [ -f "$manifest" ] || { echo "manifest not found: $manifest; run prepare first" >&2; exit 1; }; }

manifest_write() {
  local tmp="${manifest}.tmp.$$"
  jq "$@" "$manifest" > "$tmp"
  mv "$tmp" "$manifest"
}

stage_set() {
  local name=$1 status=$2 details=${3:-'{}'} at
  at=$(now)
  manifest_write --arg name "$name" --arg status "$status" --arg at "$at" --argjson details "$details" \
    '.stages[$name] = {status: $status, at: $at, details: $details} | .updated_at = $at'
}

manifest_set_string() {
  local path=$1 value=$2
  manifest_write --arg value "$value" "$path = $value"
}

manifest_details() {
  jq -cn --arg output "${1:-}" '{output: $output}'
}

ensure_manifest_source() {
  local current_commit current_version manifest_commit manifest_version
  current_commit=$(git -C "$REPO_ROOT" rev-parse HEAD)
  current_version=$repo_version
  manifest_commit=$(jq -er '.source.commit' "$manifest")
  manifest_version=$(jq -er '.source.version' "$manifest")
  [ "$current_commit" = "$manifest_commit" ] || {
    echo "source commit changed: manifest=$manifest_commit current=$current_commit" >&2
    return 1
  }
  [ "$current_version" = "$manifest_version" ] || {
    echo "source VERSION changed: manifest=$manifest_version current=$current_version" >&2
    return 1
  }
}

repo_branch=$(git -C "$REPO_ROOT" branch --show-current)
repo_version=$(tr -d '\r\n' < "$REPO_ROOT/backend/cmd/server/VERSION")

repo_preflight() {
  [ "$repo_branch" = main ] || { echo "branch=$repo_branch expected=main" >&2; return 1; }
  [ -z "$(git -C "$REPO_ROOT" diff --name-only --diff-filter=U)" ] || {
    echo 'unmerged paths exist' >&2
    git -C "$REPO_ROOT" diff --name-only --diff-filter=U >&2 || true
    return 1
  }
  [ -z "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=normal)" ] || {
    echo 'working tree is not clean' >&2
    return 1
  }
  origin_actual=$(git -C "$REPO_ROOT" remote get-url origin)
  [ "$origin_actual" = "$expected_origin" ] || { echo "origin=$origin_actual expected=$expected_origin" >&2; return 1; }
  upstream_actual=$(git -C "$REPO_ROOT" remote get-url upstream)
  [ "$upstream_actual" = "$official_remote" ] || { echo "upstream=$upstream_actual expected=$official_remote" >&2; return 1; }
  [ "$release_verified" = true ] && [ "$source_verified" = true ] || {
    echo 'workspace version verification is not enabled' >&2
    return 1
  }
  [ "$repo_version" = "$expected_version" ] && [ "$repo_version" = "$verified_source_version" ] || {
    echo "VERSION=$repo_version expected=$expected_version verified=$verified_source_version" >&2
    return 1
  }
}

fetch_origin() {
  retry_with_backoff "fetch origin main" git -C "$REPO_ROOT" fetch origin main
}

ssh_options=(-o BatchMode=yes -o ConnectTimeout=10 -o ServerAliveInterval=5 -o ServerAliveCountMax=3)

remote_snapshot() {
  local output
  output=$(retry_with_backoff "production status" ssh "${ssh_options[@]}" "$ssh_alias" bash -s -- \
    "$deploy_root" "$app_service" "${preserved_services[@]}" <<'REMOTE'
set -Eeuo pipefail
deploy_root=$1
app_service=$2
shift 2
preserved=("$@")
sudo -n true
normalize_arch() {
  case "$1" in
    aarch64|arm64) echo arm64 ;;
    x86_64|amd64) echo amd64 ;;
    *) echo "$1" ;;
  esac
}
inspect_state() {
  local name=$1
  sudo -n docker inspect "$name" --format '{{.State.Status}}'
}
inspect_health() {
  local name=$1
  sudo -n docker inspect "$name" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}'
}
printf 'REMOTE_ARCH=%s\n' "$(normalize_arch "$(uname -m)")"
printf 'DOCKER_ARCH=%s/%s\n' "$(sudo -n docker version --format '{{.Server.Os}}')" "$(sudo -n docker version --format '{{.Server.Arch}}')"
printf 'APP_IMAGE=%s\n' "$(sudo -n docker inspect "$app_service" --format '{{.Config.Image}}')"
printf 'APP_STATUS=%s\n' "$(inspect_state "$app_service")"
printf 'APP_HEALTH=%s\n' "$(inspect_health "$app_service")"
printf 'APP_MOUNTS=%s\n' "$(sudo -n docker inspect "$app_service" --format '{{range .Mounts}}{{.Source}}->{{.Destination}};{{end}}')"
for name in "${preserved[@]}"; do
  printf 'PRESERVED_STATUS_%s=%s\n' "$name" "$(inspect_state "$name")"
  printf 'PRESERVED_MOUNTS_%s=%s\n' "$name" "$(sudo -n docker inspect "$name" --format '{{range .Mounts}}{{.Source}}->{{.Destination}};{{end}}')"
done
printf 'DEPLOY_ROOT=%s\n' "$deploy_root"
REMOTE
  )
  printf '%s\n' "$output"
}

remote_value() {
  local key=$1 output=${2:-$remote_output}
  printf '%s\n' "$output" | sed -n "s/^${key}=//p" | tail -n 1
}

public_health_snapshot() {
  mapfile -t health_paths < <(jq -er '.production.healthPaths[]' "$CONFIG")
  local path url body
  public_health_output=''
  for path in "${health_paths[@]}"; do
    url="${public_base_url%/}${path}"
    body=$(retry_with_backoff "public health $path" curl --fail --silent --show-error --max-time 20 "$url")
    public_health_output+="${path}=${body}"$'\n'
  done
  printf '%s' "$public_health_output"
}

record_remote_status() {
  local details
  details=$(jq -cn --arg remote "$remote_output" --arg public "${public_health_output:-}" \
    '{remote_output: $remote, public_health_output: $public}')
  manifest_write --arg at "$(now)" --argjson details "$details" \
    '.observations.last_status = {at: $at, details: $details} | .updated_at = $at'
}

validate_remote() {
  [ "$(remote_value REMOTE_ARCH)" = "$target_architecture" ] || { echo 'remote architecture mismatch' >&2; return 1; }
  [ "$(remote_value DOCKER_ARCH)" = 'linux/arm64' ] || { echo 'remote Docker architecture mismatch' >&2; return 1; }
  [ "$(remote_value APP_STATUS)" = running ] || { echo 'application is not running' >&2; return 1; }
  local app_health
  app_health=$(remote_value APP_HEALTH)
  [ "$app_health" = healthy ] || { echo "application health=$app_health" >&2; return 1; }
  local service status mounts mount_evidence=0
  for service in "${preserved_services[@]}"; do
    status=$(remote_value "PRESERVED_STATUS_${service}")
    [ "$status" = running ] || { echo "$service status=$status" >&2; return 1; }
    mounts=$(remote_value "PRESERVED_MOUNTS_${service}")
    if [ -n "$mounts" ]; then
      mount_evidence=1
    else
      echo "$service has no mounts; preserving its running state" >&2
    fi
  done
  [ "$mount_evidence" -eq 1 ] || { echo 'no preserved-service mount evidence found' >&2; return 1; }
}

verify_remote() {
  remote_output=$(remote_snapshot)
  public_health_output=$(public_health_snapshot)
  record_remote_status
  validate_remote
}

prepare() {
  require_release
  [ -n "$tag" ] || tag="${project_image}:preflight-v${repo_version}-arm64"
  repo_preflight
  if [ -f "$manifest" ]; then
    existing_tag=$(jq -r '.artifact.source_tag // empty' "$manifest")
    [ "$existing_tag" = "$tag" ] || { echo "existing release uses tag=$existing_tag, requested tag=$tag" >&2; return 1; }
    ensure_manifest_source
    echo "manifest_exists=$manifest"
    return 0
  fi
  mkdir -p "$release_dir"
  local commit
  commit=$(git -C "$REPO_ROOT" rev-parse HEAD)
  jq -n \
    --arg release "$release" \
    --arg created "$(now)" \
    --arg commit "$commit" \
    --arg version "$repo_version" \
    --arg tag "$tag" \
    --arg project_image "$project_image" \
    --arg target "$target_platform" \
    --arg ssh "$ssh_alias" \
    --arg app_service "$app_service" \
    '{schemaVersion: 1, release: $release, created_at: $created, updated_at: $created, source: {commit: $commit, version: $version, branch: "main"}, artifact: {source_tag: $tag, release_image: ($project_image + ":" + $release), target: $target}, production: {ssh_alias: $ssh, app_service: $app_service}, stages: {}, observations: {}}' \
    > "$manifest"
  stage_set prepare passed "$(jq -cn --arg tag "$tag" --arg commit "$commit" '{tag:$tag,commit:$commit}')"
  echo "manifest=$manifest"
}

verify() {
  require_manifest
  repo_preflight
  ensure_manifest_source
  retry_with_backoff "fetch origin main" git -C "$REPO_ROOT" fetch origin main
  local behind ahead
  read -r behind ahead < <(git -C "$REPO_ROOT" rev-list --left-right --count origin/main...HEAD)
  [ "$behind" = 0 ] || { echo "local main is behind origin/main by $behind commit(s)" >&2; stage_set verify blocked "$(jq -cn --arg behind "$behind" '{behind:$behind}')"; return 1; }
  remote_output=''
  public_health_output=''
  if ! verify_remote; then
    stage_set verify blocked "$(jq -cn --arg output "${remote_output:-}" --arg public "${public_health_output:-}" '{remote_output:$output,public_health_output:$public}')"
    return 1
  fi
  stage_set verify passed "$(jq -cn --arg behind "$behind" --arg ahead "$ahead" --arg commit "$(git -C "$REPO_ROOT" rev-parse HEAD)" '{behind:$behind,ahead:$ahead,commit:$commit}')"
  echo "verify=passed commit=$(git -C "$REPO_ROOT" rev-parse HEAD)"
}

build() {
  require_manifest
  ensure_manifest_source
  local source_tag
  source_tag=$(jq -er '.artifact.source_tag' "$manifest")
  local arch image_id
  if ! arch=$(docker image inspect "$source_tag" --format '{{.Os}}/{{.Architecture}}' 2>/dev/null); then
    (cd "$REPO_ROOT" && bash "$SCRIPT_DIR/build-arm64.sh" --tag "$source_tag")
    arch=$(docker image inspect "$source_tag" --format '{{.Os}}/{{.Architecture}}')
  else
    echo "existing_image=$source_tag architecture=$arch"
  fi
  [ "$arch" = 'linux/arm64' ] || { echo "image architecture=$arch expected=linux/arm64" >&2; stage_set build blocked "$(jq -cn --arg arch "$arch" '{architecture:$arch}')"; return 1; }
  image_id=$(docker image inspect "$source_tag" --format '{{.Id}}')
  manifest_write --arg id "$image_id" --arg arch "$arch" --arg at "$(now)" \
    '.artifact.image_id = $id | .artifact.architecture = $arch | .updated_at = $at'
  stage_set build passed "$(jq -cn --arg id "$image_id" --arg arch "$arch" '{image_id:$id,architecture:$arch}')"
  echo "build=passed image=$source_tag architecture=$arch id=$image_id"
}

publish() {
  require_manifest
  ensure_manifest_source
  [ "$(jq -r '.stages.verify.status // ""' "$manifest")" = passed ] || { echo 'verify stage must pass before publish' >&2; return 1; }
  [ "$(jq -r '.stages.build.status // ""' "$manifest")" = passed ] || { echo 'build stage must pass before publish' >&2; return 1; }
  local behind ahead
  retry_with_backoff "fetch origin main" git -C "$REPO_ROOT" fetch origin main
  read -r behind ahead < <(git -C "$REPO_ROOT" rev-list --left-right --count origin/main...HEAD)
  [ "$behind" = 0 ] || { echo "cannot publish while local main is behind origin/main by $behind" >&2; return 1; }
  if [ "$ahead" -gt 0 ]; then
    retry_with_backoff "push origin main" git -C "$REPO_ROOT" push origin main
  else
    echo 'origin/main already contains local HEAD'
  fi
  retry_with_backoff "verify origin main" git -C "$REPO_ROOT" fetch origin main
  [ "$(git -C "$REPO_ROOT" rev-parse HEAD)" = "$(git -C "$REPO_ROOT" rev-parse origin/main)" ] || {
    echo 'origin/main does not match local HEAD' >&2
    return 1
  }
  stage_set publish passed "$(jq -cn --arg commit "$(git -C "$REPO_ROOT" rev-parse HEAD)" '{commit:$commit}')"
  echo "publish=passed commit=$(git -C "$REPO_ROOT" rev-parse HEAD)"
}

plan() {
  require_manifest
  ensure_manifest_source
  [ "$(jq -r '.stages.publish.status // ""' "$manifest")" = passed ] || { echo 'publish stage must pass before plan' >&2; return 1; }
  local source_tag release_image plan_log status
  source_tag=$(jq -er '.artifact.source_tag' "$manifest")
  release_image=$(jq -er '.artifact.release_image' "$manifest")
  plan_log="$release_dir/plan.log"
  set +e
  bash "$SCRIPT_DIR/deploy-arm64.sh" --tag "$source_tag" --release "$release" 2>&1 | tee "$plan_log"
  status=${PIPESTATUS[0]}
  set -e
  if [ "$status" -ne 0 ]; then
    stage_set plan blocked "$(jq -cn --arg log "$plan_log" --arg status "$status" '{log:$log,exit_code:($status|tonumber)}')"
    return "$status"
  fi
  manifest_write --arg release_image "$release_image" --arg log "$plan_log" --arg at "$(now)" \
    '.observations.plan = {release_image: $release_image, log: $log, at: $at} | .updated_at = $at'
  stage_set plan passed "$(jq -cn --arg log "$plan_log" '{log:$log}')"
  echo "plan=passed release_image=$release_image"
}

apply_release() {
  require_manifest
  ensure_manifest_source
  [ "$(jq -r '.stages.plan.status // ""' "$manifest")" = passed ] || { echo 'plan stage must pass before apply' >&2; return 1; }
  local source_tag log status
  source_tag=$(jq -er '.artifact.source_tag' "$manifest")
  log="$release_dir/apply.log"
  if ! verify_remote; then
    stage_set apply blocked "$(jq -cn --arg output "${remote_output:-}" --arg public "${public_health_output:-}" '{remote_output:$output,public_health_output:$public}')"
    return 1
  fi
  manifest_write --arg at "$(now)" '.updated_at = $at | .observations.apply_started_at = $at'
  stage_set apply started "$(jq -cn --arg log "$log" '{log:$log}')"
  set +e
  bash "$SCRIPT_DIR/deploy-arm64.sh" --tag "$source_tag" --release "$release" --apply 2>&1 | tee "$log"
  status=${PIPESTATUS[0]}
  set -e
  if [ "$status" -eq 0 ]; then
    stage_set apply command_passed "$(jq -cn --arg log "$log" '{log:$log}')"
  else
    stage_set apply command_failed "$(jq -cn --arg log "$log" --arg status "$status" '{log:$log,exit_code:($status|tonumber)}')"
  fi
  if ! verify_remote; then
    stage_set apply uncertain "$(jq -cn --arg output "${remote_output:-}" --arg public "${public_health_output:-}" '{remote_output:$output,public_health_output:$public}')"
    echo "apply result is uncertain; inspect with: tools/sub2api-release.sh status --release $release" >&2
    [ "$status" -ne 0 ] && return "$status"
    return 1
  fi
  local current_image target_image
  current_image=$(remote_value APP_IMAGE)
  target_image=$(jq -er '.artifact.release_image' "$manifest")
  if [ "$current_image" = "$target_image" ] && [ "$status" -eq 0 ]; then
    stage_set apply succeeded "$(jq -cn --arg image "$current_image" '{image:$image}')"
    stage_set success_verified passed "$(jq -cn --arg image "$current_image" '{image:$image}')"
    echo "apply=passed image=$current_image"
    return 0
  fi
  if [ "$current_image" = "$target_image" ]; then
    stage_set apply succeeded_after_transport_error "$(jq -cn --arg image "$current_image" --arg status "$status" '{image:$image,command_exit_code:($status|tonumber)}')"
    stage_set success_verified passed "$(jq -cn --arg image "$current_image" '{image:$image}')"
    echo "apply=verified_after_transport_error image=$current_image"
    return 0
  fi
  stage_set apply not_switched "$(jq -cn --arg image "$current_image" --arg target "$target_image" --arg status "$status" '{current_image:$image,target_image:$target,command_exit_code:($status|tonumber)}')"
  echo "apply did not leave target image active; current_image=$current_image target_image=$target_image" >&2
  return 1
}

status_command() {
  require_manifest
  remote_output=$(remote_snapshot)
  public_health_output=$(public_health_snapshot)
  record_remote_status
  printf '%s\n' "$remote_output"
  printf '%s' "$public_health_output"
  printf 'manifest=%s\n' "$manifest"
}

recover() {
  require_manifest
  if ! status_command; then
    stage_set apply uncertain "$(jq -cn --arg reason 'remote status unavailable' '{reason:$reason}')"
    return 1
  fi
  if ! validate_remote; then
    stage_set apply uncertain "$(jq -cn --arg reason 'post-status verification failed' '{reason:$reason}')"
    return 1
  fi
  local current_image target_image app_health
  current_image=$(remote_value APP_IMAGE)
  target_image=$(jq -er '.artifact.release_image' "$manifest")
  app_health=$(remote_value APP_HEALTH)
  if [ "$current_image" = "$target_image" ] && [ "$app_health" = healthy ]; then
    stage_set apply recovered_success "$(jq -cn --arg image "$current_image" '{image:$image}')"
    stage_set success_verified passed "$(jq -cn --arg image "$current_image" '{image:$image}')"
    echo 'recover=success'
  elif [ "$app_health" = healthy ]; then
    stage_set apply recovered_not_switched "$(jq -cn --arg image "$current_image" --arg target "$target_image" '{current_image:$image,target_image:$target}')"
    echo "recover=not_switched current_image=$current_image target_image=$target_image"
    return 1
  else
    stage_set apply uncertain "$(jq -cn --arg image "$current_image" --arg health "$app_health" '{current_image:$image,health:$health}')"
    echo 'recover=uncertain' >&2
    return 1
  fi
}

run_all() {
  require_release
  prepare
  verify
  build
  publish
  plan
  if [ "$apply" -eq 1 ]; then
    apply_release
  else
    echo 'run=plan-only; use apply --release with the same release name after review'
  fi
}

case "$command_name" in
  prepare) prepare ;;
  verify) verify ;;
  build) build ;;
  publish) publish ;;
  plan) plan ;;
  apply) apply_release ;;
  status) status_command ;;
  recover) recover ;;
  run) run_all ;;
  -h|--help) usage ;;
  *) echo "unknown command: $command_name" >&2; usage >&2; exit 2 ;;
esac

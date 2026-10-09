#!/usr/bin/env bash

verify_release_source() {
  local official_tag official_commit source_commit source_version expected_version project_version status actual_commit actual_version merge_commit
  official_tag=$(jq -er '.versionVerification.latestVerifiedTag' "$CONFIG") || return
  official_commit=$(jq -er '.versionVerification.latestVerifiedCommit' "$CONFIG") || return
  source_commit=$(jq -er '.versionVerification.verifiedSourceCommit' "$CONFIG") || return
  source_version=$(jq -er '.versionVerification.verifiedSourceVersionFile' "$CONFIG") || return
  expected_version=$(jq -er '.versionVerification.expectedTagVersion' "$CONFIG") || return
  project_version=$(jq -er '.versionVerification.projectVersion // .versionVerification.expectedTagVersion' "$CONFIG") || return
  status=$(jq -er '.versionVerification.status' "$CONFIG") || return
  if ! jq -e '.versionVerification.sourceVerified == true and .versionVerification.releaseVersionVerified == true' "$CONFIG" >/dev/null ||
    [ "$status" != verified ] || [ "$official_tag" != "v$expected_version" ] || [ "$project_version" != "$expected_version" ]; then
    echo 'official release verification is incomplete or inconsistent' >&2
    return 3
  fi
  actual_commit=$(git -C "$REPO_ROOT" rev-parse --verify "refs/tags/$official_tag^{commit}") || return
  [ "$actual_commit" = "$official_commit" ] && [ "$source_commit" = "$official_commit" ] || {
    echo "official tag/source commit does not match verified commit $official_commit" >&2
    return 3
  }
  actual_version=$(git -C "$REPO_ROOT" show "$source_commit:backend/cmd/server/VERSION" | tr -d '\r\n') || return
  [ "$actual_version" = "$source_version" ] && [ "$actual_version" = "$(jq -er '.versionVerification.tagVersionFile' "$CONFIG")" ] || {
    echo "official source VERSION=$actual_version differs from workspace evidence=$source_version" >&2
    return 3
  }
  [ "$(tr -d '\r\n' < "$REPO_ROOT/backend/cmd/server/VERSION")" = "$project_version" ] || {
    echo "project VERSION must be $project_version (official source VERSION is $source_version)" >&2
    return 3
  }
  if ! git -C "$REPO_ROOT" merge-base --is-ancestor "$source_commit" HEAD; then
    merge_commit=$(git -C "$REPO_ROOT" rev-parse --verify 'MERGE_HEAD^{commit}' 2>/dev/null) || {
      echo "source does not include official release $official_tag" >&2
      return 3
    }
    git -C "$REPO_ROOT" merge-base --is-ancestor "$source_commit" "$merge_commit" || {
      echo "pending merge does not include official release $official_tag" >&2
      return 3
    }
  fi
}

image_revision() {
  local revision
  revision=$(git -C "$REPO_ROOT" rev-parse HEAD) || return
  if [ -n "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=normal)" ]; then
    revision="$revision-dirty"
  fi
  printf '%s\n' "$revision"
}

verify_release_image() {
  local image=$1 expected_revision=$2 metadata
  metadata=$(docker image inspect "$image" --format '{{json .}}') || return
  jq -e --arg platform "$(jq -er '.target.platform' "$CONFIG")" \
    --arg version "$(jq -er '.versionVerification.projectVersion // .versionVerification.expectedTagVersion' "$CONFIG")" \
    --arg revision "$expected_revision" \
    '(.Os + "/" + .Architecture) == $platform and
     .Config.Labels["org.opencontainers.image.version"] == $version and
     .Config.Labels["org.opencontainers.image.revision"] == $revision' <<< "$metadata" >/dev/null || {
    echo "image $image does not match the configured platform, release version, and source revision" >&2
    return 3
  }
}

# ARM64 Upgrade and Release

This repository separates source maintenance from production release. Semantic merge conflicts remain a human review gate. The repeatable build, Git synchronization, deployment, health verification, and recovery steps are driven by `workspace.json` and a release manifest.

## 1. Prepare the official source

Run from `sub2api-custom` on a clean `main` branch:

```bash
bash tools/upgrade-arm64.sh
bash tools/upgrade-arm64.sh --merge
```

Resolve conflicts deliberately. Preserve local custom behavior and intentional deletions. Confirm `backend/cmd/server/VERSION` matches the verified source version in `workspace.json`, run relevant tests, and commit the result. The script never guesses through semantic conflicts.

## 2. Create a release manifest

The staged release entry point is `tools/sub2api-release.sh`. It records source commit, version, image, stage results, logs, remote observations, and recovery state in:

```text
deploy-artifacts/<release>/release.json
```

Choose one release name and one immutable Git tag, then reuse both for every later command:

```bash
version=$(tr -d '\r\n' < backend/cmd/server/VERSION)
tag="sub2api-custom:preflight-v${version}-arm64"
release="${version}-arm64-$(date -u +%Y%m%d-%H%M%S)"
git_tag="release-${version}-${release}"
```

Validate the tag name before preparing the release:

```bash
git check-ref-format "refs/tags/$git_tag"
```

`prepare` requires `--git-tag`. On a clean `main`, it creates an annotated tag at the current `HEAD` when the tag is absent. An existing tag is accepted only when it already resolves to the current `HEAD`; the release tool never moves a tag to another commit.

Create the manifest and bind it to the clean source commit:

```bash
bash tools/sub2api-release.sh prepare --release "$release" --git-tag "$git_tag" --tag "$tag"
git rev-parse --verify "refs/tags/$git_tag^{commit}"
git rev-parse HEAD
```

The two commit IDs must match. The manifest records the tag in `source.git_tag`, and all new stages validate that it still resolves to `source.commit`.

## 3. Run staged checks

Run these stages in order:

```bash
bash tools/sub2api-release.sh verify --release "$release"
bash tools/sub2api-release.sh build --release "$release"
bash tools/sub2api-release.sh publish --release "$release"
bash tools/sub2api-release.sh plan --release "$release"
```

The gates are:

- `verify`: clean `main`, configured remotes, verified source version, `origin/main` not ahead, remote ARM64/Docker access, application health, preserved services, mount evidence, and public health.
- `build`: local image exists or is built through the configured Buildx path and is verified as `linux/arm64`.
- `publish`: pushes only the configured `origin/main` and the exact manifest tag to `origin`, then verifies both resolve to the expected commit. It never pushes `upstream`.
- To validate the remote tag after publishing, run `git ls-remote origin "refs/tags/${git_tag}^{}"` and compare its commit ID with `git rev-parse HEAD`.
- `plan`: records the deployment plan and does not upload an image or modify production.

The compatibility entry point remains available:

```bash
bash tools/release-arm64.sh --tag "$tag" --release "$release" --git-tag "$git_tag"
```

The wrapper accepts `--git-tag`; when omitted, it deterministically generates `release-${version}-${release}` and passes that value to `run`. `run` performs the same tag creation and validation before the staged checks. Add `--apply` only after reviewing the manifest and plan.

## 4. Apply production

Apply only the reviewed manifest:

```bash
bash tools/sub2api-release.sh apply --release "$release"
```

The underlying deploy script uploads and checksum-verifies the image, validates the remote architecture, resolves relative Compose files below the configured deployment root, and recreates only `sub2api`. PostgreSQL, Redis, `sub2api-leaderboard`, and their data directories are not stopped, recreated, or included in rollback commands.

A successful apply records `success_verified`. If the command exits after possibly reaching the server, the tool checks the actual remote image and health before deciding the result. It records `uncertain` when evidence is incomplete and never blindly repeats the complete service switch.

## 5. Inspect and recover

Both commands require an existing manifest and do not require a local Docker daemon:

```bash
bash tools/sub2api-release.sh status --release "$release"
bash tools/sub2api-release.sh recover --release "$release"
```

Use `status` after an SSH or transport interruption. Use `recover` to classify the observed state as `success`, `not_switched`, or `uncertain`. A `not_switched` result is not an authorization to retry blindly; review the manifest and plan first.

## Retry policy

Bounded exponential backoff is used only for idempotent Git operations, Docker builds, SSH preflight, image upload, and health checks:

- `RETRY_ATTEMPTS`, default `4`
- `RETRY_DELAY_SECONDS`, default `5`
- `RETRY_MAX_DELAY_SECONDS`, default `30`

A complete remote service switch is intentionally not blindly retried after an uncertain transport result. Inspect the manifest and production state first.

## Production boundary

All target, remote, service, Compose, image, and health values come from `workspace.json`. The production SSH path uses the configured non-interactive `sudo` authorization only. Version, source, architecture, Docker permission, preserved-service, and health gates must pass before any production write.

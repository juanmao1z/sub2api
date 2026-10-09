# ARM64 Upgrade and Release

This repository separates source maintenance from production release. Semantic merge conflicts remain a human review gate. The repeatable build, Git synchronization, deployment, health verification, and recovery steps are driven by `workspace.json` and a release manifest.

## 1. Prepare the official source

Run from `sub2api-custom` on a clean `main` branch:

```bash
bash tools/upgrade-arm64.sh
bash tools/upgrade-arm64.sh --merge
```

Resolve conflicts deliberately. Preserve local custom behavior and intentional deletions. Confirm `backend/cmd/server/VERSION` matches `versionVerification.projectVersion` in `workspace.json`, run relevant tests, and commit the result. The script never guesses through semantic conflicts.

The default source is the verified release tag, not moving `upstream/main`. Its peeled commit and original VERSION must match `workspace.json`. Official `v0.2.15` points to `f2669c8cf62555cd92389b3f55920e9e6e7c6ff2`, but its VERSION file still says `0.2.14`. The workspace records that original value in `verifiedSourceVersionFile` and the custom release value `0.2.15` in `projectVersion`. The build passes VERSION explicitly; this mismatch is not treated as unverified source.

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
- `build`: reuses an image only when architecture, OCI version and source revision match the manifest; otherwise rebuilds it through the current Buildx path. Dirty local validation images cannot be published or deployed as committed releases.
- `publish`: pushes only the configured `origin/main` and the exact manifest tag to `origin`, then verifies both resolve to the expected commit. It never pushes `upstream`.
- To validate the remote tag after publishing, run `git ls-remote origin "refs/tags/${git_tag}^{}"` and compare its commit ID with `git rev-parse HEAD`.
- `plan`: records the deployment plan and does not upload an image or modify production.
- `upload`: an explicit remote write, available after a reviewed plan. It checks the server and Compose configuration before uploading, then verifies SHA256, image ID, ARM64 architecture, version and revision after loading. It never recreates a service.

The compatibility entry point remains available:

```bash
bash tools/release-arm64.sh --tag "$tag" --release "$release" --git-tag "$git_tag"
```

The wrapper accepts `--git-tag`; when omitted, it deterministically generates `release-${version}-${release}` and passes that value to `run`. `run` performs the same tag creation and validation before the staged checks. Add `--apply` only after reviewing the manifest and plan.

To stage the image separately without switching the application:

```bash
bash tools/sub2api-release.sh upload --release "$release"
```

`run` does not implicitly upload an image. Standalone diagnostics can use `bash tools/deploy-arm64.sh --tag "$tag" --release "$release"` for a read-only plan, or explicitly add `--upload-only` for staging. Both `--upload-only` and `--apply` reject dirty or unfinished merges before any remote write.

## 4. Apply production

Apply only the reviewed manifest:

```bash
bash tools/sub2api-release.sh apply --release "$release"
```

The underlying deploy script performs the same verified upload, then writes only the dedicated `production.releaseComposeFile` (`docker-compose.release.json`). It does not rewrite existing YAML or copy resolved configuration containing secrets into artifacts. Compose commands include this file last and use `up -d --no-deps --pull never sub2api`. Preserved-service container IDs and running states are checked after the switch. Rollback restores or removes only the dedicated override and restarts only the application.

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

## Local validation and prerequisites

```bash
node --test deploy/tests/release.test.mjs
bash -n tools/*.sh tools/lib/*.sh
bash tools/build-arm64.sh --print
```

Plans require Bash, Git, jq and the configured official tag locally. Builds additionally require a reachable Docker daemon and a Buildx builder advertising `linux/arm64`. Actual upload/apply requires SSH/SCP, curl, gzip, sha256sum, the configured SSH alias/key, non-interactive remote sudo, readable production Compose files and a healthy application. `NPM_CONFIG_REGISTRY`, Go module settings and proxy variables are forwarded only when provided by the current environment; no machine-specific endpoint or credential belongs in workspace.json. Native frontend checks require pnpm 9, or the already-installed `frontend/node_modules/.bin` tools.

After a successful deployment, manual Compose operations must also append `-f docker-compose.release.json` after the configured base files, otherwise they may restore the old base image. The release script always includes this override automatically.

## Password reset setup

The public authentication routes are `/forgot-password` and `/reset-password`. In production, set the frontend base URL to the site root:

```text
https://api.zhouz.online
```

Do not enter `/api`, `/login`, or `/reset-password`; the application appends `/reset-password` and the email/token query parameters automatically.

In Admin Settings, enable email verification, configure and test SMTP, enable password reset, set **Frontend URL** to `https://api.zhouz.online`, and save the settings. SMTP credentials and the email verification/password reset switches are stored in Admin Settings; do not commit passwords or tokens.

The server configuration can provide the same frontend URL as a fallback:

```yaml
server:
  frontend_url: "https://api.zhouz.online"
```

The database value saved from Admin Settings takes precedence over the configuration-file fallback. Changing the server configuration requires restarting `sub2api`; this documentation change does not perform a production restart or deployment.

---
name: sub2api-arm64-upgrade
description: Use when a user asks to upgrade a config-driven Sub2API deployment, merge a newer official source while preserving custom changes, build a linux/arm64 image, push origin/main, or upload and deploy only the application service with rollback and health verification.
compatibility: Requires Bash, Git, Docker Buildx, jq, SSH/SCP, curl, and the repository tools under tools/.
---

# Sub2API ARM64 Upgrade

Use this skill for the complete, config-driven Sub2API upgrade and release workflow. It is designed for a repository containing `workspace.json`, `tools/upgrade-arm64.sh`, `tools/build-arm64.sh`, `tools/sub2api-release.sh`, `tools/release-arm64.sh`, and `tools/deploy-arm64.sh`.

## Safety contract

- Read `workspace.json` before making decisions. It is the source of truth for target platform, official source, origin remote, production SSH alias, deployment root, preserved services, Compose files, and health paths.
- Preserve existing user changes. Do not reset, checkout, force-push, rewrite history, or clean unrelated repositories.
- Push only the configured `origin`; never push `upstream`.
- Treat semantic merge conflicts as human-review gates. Never resolve them by guessing.
- Do not write credentials, tokens, private keys, `.env` files, or server secrets into the workspace or skill package.
- Production writes are blocked until source version, image architecture, remote architecture, Docker access, preserved-service state, and public health prerequisites are verified.
- Production apply may recreate only the configured application service. PostgreSQL, Redis, leaderboard, and configured data mounts must remain untouched.

## Manifest and gates

The release manifest at `deploy-artifacts/<release>/release.json` is the source of truth for a resumable release. It records the source commit and version, image tag, stage status, logs, remote observations, and recovery result.

Advance only when the named evidence exists:

1. `prepare`: clean `main`, configured remotes, verified source version, and a manifest bound to one source commit.
2. `verify`: local `HEAD` is not behind `origin/main`; remote is ARM64 with authorized Docker access; application and preserved services are running; mount evidence and public health are present.
3. `build`: the image exists or is built through the configured toolchain and reports `linux/arm64`.
4. `publish`: only configured `origin/main` is pushed, then local `HEAD` and `origin/main` are compared.
5. `plan`: deployment plan is recorded without production image upload or service change.
6. `apply`: only the configured application service is switched; command result is checked against a fresh remote snapshot.
7. `success_verified`: target image, application health, preserved services, mount evidence, and public health all pass.

Never claim a later gate without evidence for earlier gates.

## Workflow

### 1. Resolve and update source

Run from the repository root on a clean `main` branch:

```bash
bash tools/upgrade-arm64.sh
bash tools/upgrade-arm64.sh --merge
```

The command fetches the official source with bounded backoff and checks its version. Resolve conflicts deliberately, preserving local custom behavior and intentional deletions. Verify `backend/cmd/server/VERSION`, run relevant tests, and commit the result.

If fetching fails after retries, stop and report the transport failure. If merging stops with conflicts, report the conflict files and do not continue to release.

### 2. Create and advance one manifest

Choose one immutable release name and image tag:

```bash
version=$(tr -d '\r\n' < backend/cmd/server/VERSION)
tag="sub2api-custom:preflight-v${version}-arm64"
release="${version}-arm64-$(date -u +%Y%m%d-%H%M%S)"
```

Create the manifest:

```bash
bash tools/sub2api-release.sh prepare --release "$release" --tag "$tag"
```

Advance it in order:

```bash
bash tools/sub2api-release.sh verify --release "$release"
bash tools/sub2api-release.sh build --release "$release"
bash tools/sub2api-release.sh publish --release "$release"
bash tools/sub2api-release.sh plan --release "$release"
```

Each later stage checks that the current source commit and version still match the manifest. `prepare` is idempotent only for the same tag, source commit, and source version. Use a new release name after changing source.

The compatibility wrapper is plan-only by default:

```bash
bash tools/release-arm64.sh --tag "$tag" --release "$release"
```

It forwards to `tools/sub2api-release.sh run`, which executes the same stages. Add `--apply` only after reviewing the manifest and plan.

### 3. Apply and verify production

Apply the reviewed release:

```bash
bash tools/sub2api-release.sh apply --release "$release"
```

The deploy script uploads a checksum-verified image, checks remote architecture, resolves relative Compose files below the configured deployment root, and recreates only the application service. It rolls back the application override if the switch or health check fails.

After apply, the manifest must contain evidence for:

- application image, state, and health;
- PostgreSQL, Redis, and leaderboard state;
- data-mount evidence;
- configured public health response;
- local `HEAD` versus `origin/main`.

A command failure followed by an unreachable or contradictory remote state is `uncertain`. A healthy old image is `not_switched`; it is not proof of rollback. Do not rerun a complete service switch until the state is inspected.

### 4. Inspect and recover

These commands are read-only with respect to production services and do not require a local Docker daemon:

```bash
bash tools/sub2api-release.sh status --release "$release"
bash tools/sub2api-release.sh recover --release "$release"
```

Use `status` after transport interruption. `recover` reuses that single remote snapshot and classifies the result as `success`, `not_switched`, or `uncertain`. It never blindly repeats apply.

## Failure handling

Use `references/recovery.md` for the recovery matrix. Retry only bounded, idempotent Git, build, SSH preflight, upload, and health operations. Verify postconditions after retries. Keep official tag metadata separate from deployable source commit/version. Use only already-authorized non-interactive `sudo`. Stop when evidence is missing.

## Final report

Report the exact source commit, image tag, architecture, release name, archive checksum when an apply occurred, application status, preserved-service status, mount evidence, public health response, and retry or rollback events. State explicitly whether the result was plan-only, uploaded, deployed, not switched, uncertain, or rolled back.

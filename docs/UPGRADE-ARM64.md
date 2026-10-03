# ARM64 Upgrade and Release

This repository keeps the source merge and the production release as separate
steps. The merge can contain semantic conflicts because local custom features
and intentional deletions must be reviewed by a human. The repeatable network,
build, push, upload, and deployment steps are scripted.

## 1. Prepare the official source

Run from `sub2api-custom` on a clean `main` branch:

```bash
bash tools/upgrade-arm64.sh
bash tools/upgrade-arm64.sh --merge
```

Resolve conflicts deliberately. Keep the local custom behavior and confirm the
source `backend/cmd/server/VERSION` matches the verified version in
`workspace.json`. Run the relevant tests, then commit the merge. The script
never guesses through semantic conflicts.

## 2. Release and deploy

The release script verifies the remote URL, branch, clean tree, version gate,
ARM64 builder, image architecture, and GitHub synchronization. It retries
network-sensitive Git operations and delegates image construction and deployment
to the existing checked scripts.

Set one release name and reuse it for the plan and apply steps:

version=$(tr -d '\r\n' < backend/cmd/server/VERSION)
tag="sub2api-custom:preflight-v${version}-arm64"
release="${version}-arm64-$(date -u +%Y%m%d-%H%M%S)"

Plan only:

```bash
RETRY_ATTEMPTS=5 RETRY_DELAY_SECONDS=8 \
  bash tools/release-arm64.sh --tag "$tag" --release "$release"
```

Apply the same reviewed release:

```bash
RETRY_ATTEMPTS=5 RETRY_DELAY_SECONDS=8 \
  bash tools/release-arm64.sh --tag "$tag" --release "$release" --apply
```

`--apply` only recreates the configured `sub2api` service. PostgreSQL, Redis,
leaderboard, and their configured data mounts are not recreated. The deploy
script uploads a checksum-verified image, checks the remote architecture, waits
for the application health check, and restores the previous application
configuration if the switch or public `/health` check fails.

## Network retry policy

The scripts use exponential backoff for Git fetch/push, Docker image builds,
SSH preflight, SCP upload, public health checks, and rollback health checks. Tune
these environment variables when the network is unstable:

- `RETRY_ATTEMPTS`, default `4`
- `RETRY_DELAY_SECONDS`, default `5`
- `RETRY_MAX_DELAY_SECONDS`, default `30`

A complete remote deployment command is intentionally not blindly retried after
it may have reached the server. The script checks and reports the remote state
instead of risking a second uncontrolled service switch.

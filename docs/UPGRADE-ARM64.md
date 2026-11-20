# ARM64 Upgrade and Release

This repository separates source maintenance from production release. Semantic merge conflicts remain a human review gate. The repeatable build, Git synchronization, deployment, health verification, and recovery steps are driven by `workspace.json` and a release manifest.

## 1. Prepare the official source

Run from `sub2api-custom` on a clean `main` branch:

```bash
bash tools/upgrade-arm64.sh
bash tools/upgrade-arm64.sh --merge
```

Resolve conflicts deliberately. Preserve local custom behavior and intentional deletions. Confirm `backend/cmd/server/VERSION` matches `versionVerification.projectVersion` in `workspace.json`, run relevant tests, and commit the result. The script never guesses through semantic conflicts.

The default source is the verified release tag, not moving `upstream/main`. Its peeled commit and original VERSION must match `workspace.json`. Official `v0.2.24` points to `5c2982a81b46e2142d0143d2f83c1ce9c4da1d24`, but its VERSION file still says `0.2.23`. The workspace records that original value in `verifiedSourceVersionFile` and the custom release value `0.2.24` in `projectVersion`. Both build entry points use Go 1.27.4; production builds pass VERSION explicitly.

### v0.2.17 to v0.2.19 compatibility

Startup applies migrations 246 (enable image generation for Gemini and Antigravity groups), 247 (scoped administrator API keys), and 248 (group image-count billing). The custom support-ticket migration 243 is retained. Take a restorable PostgreSQL backup before a production switch: automatic rollback changes the application image and Compose override, and does not undo migrations or restore database contents. Migration 246 changes existing Gemini and Antigravity groups to allow image generation, including soft-deleted groups, so review those groups after upgrading; composite groups are outside this migration.

The pinned console, canvas, usage guide and channel-health colors remain in use. New official administrator-key management, Grok catalog diagnostics and image-count billing controls are not added to this fixed console. The backend includes these APIs; omitted `image_count_billing` updates preserve the value for supported image-generation groups, and the new setting defaults to false. Use the API contract in `docs/admin-api-keys.md` when managing scoped keys; this upgrade does not enable image-count billing automatically.

No new mandatory production environment variable is introduced. `gateway.max_same_account_retries_gemini` defaults to 4 (0–10). Removed Gemini defaults require explicit model mappings or allowlists if an account still needs those model names. Existing database, Redis, JWT and encryption configuration remains necessary. Image staging (`upload`) does not run database migrations; only starting the new application does.

### v0.2.19 to v0.2.21 compatibility

The custom migration ledger grows from 306 to 309 files: `249_add_usage_logs_api_key_cost_index_notx.sql` creates a concurrent covering index on usage logs; `249_mcp_oauth.sql` creates three OAuth tables; `250_openai_cache_write_ttl.sql` adds 30m/12h token counters and nullable 12h cache-write prices. Filenames are ledger keys, so both 249 files and all historical custom migrations remain unchanged. Index creation can extend first startup; validate against a disposable restored backup before switching production. Application rollback does not undo these migrations.

No mandatory production environment change is required. MCP remains disabled unless `MCP_PUBLIC_URL` is explicitly configured; its new management and consent screens are not included in the pinned console. The correct memory budget key is `gateway.request_body_memory_budget_bytes`; when unset, upstream selects one quarter of the smallest detected memory limit with a 1GiB floor. Existing explicit configuration takes precedence.

OpenAI cache-write billing now distinguishes 30m and 12h, with new nullable pricing fields preserving configured generic cache-write prices. The fixed console does not add the new TTL detail fields, MCP pages, image-size tier controls or new account refresh UI. Existing APIs, billing, support tickets, canvas, guides and channel colors remain. `/`, `/home` and `/login` continue using the restored historical public entry with precompiled translations under strict production CSP. Production validation must include those real routes, not only `/health`.

### v0.2.21 to v0.2.22 compatibility

The custom ledger grows from 309 to 311 files. `251_group_force_openai_ultrafast.sql` adds a false-by-default group flag; `251_group_usage_rollup_nonblocking_build.sql` adds rollup expiry/build state and replaces invalidation functions to avoid blocking usage writes during rebuilds. Preserve all historical filenames and checksums. Validate the upgrade on an actual restored production backup before switching; image rollback does not revert database migrations.

The fixed console omits `force_openai_ultrafast`; backend updates preserve the existing value when omitted, and existing groups remain disabled after migration. New Ultra Fast controls, Vertex multi-region selectors and passthrough filtering UI require separate frontend work. Existing interfaces and the removal of the homepage community dialog stay in place. HTML caching retains the custom settings TTL and fallback while applying the upstream no-store and per-request CSP nonce fixes on every cached path. No mandatory production configuration change is introduced; MCP stays disabled without `MCP_PUBLIC_URL`.

### v0.2.22 to v0.2.23 compatibility

The custom ledger grows from 311 to 312 files. `252_channel_pricing_ultrafast_multiplier.sql` adds a nullable positive Ultra Fast multiplier to channel model prices. Existing price fields and group policies remain unchanged; NULL selects model-default Ultra Fast pricing. Verify the migration against an actual restored production backup, including historical checksums, channel prices, row counts, repeat execution and positive-value constraints.

The pinned console and historical public pages remain in place, including the removed community dialog. Its build-time channel editor adaptation round-trips `ultrafast_multiplier` when loading and saving prices, preserving API-configured overrides without adding new controls. New GPT-6.2 metadata, cache-creation usage normalization and bounded first-output stream failover are included in the backend. The upstream memory-test isolation supersedes the equivalent local test workaround. No mandatory production environment change is introduced; MCP and Ultra Fast are not automatically enabled.

### v0.2.23 to v0.2.24 compatibility

No new SQL migration is introduced; the custom ledger stays at 312 files. Verify a fresh production backup by restoring it and invoking the new version's migration entry twice. The ledger checksums, user records, tickets, channel prices, platform quotas and settings must remain unchanged.

Gateway requests with duplicate JSON fields are rejected with HTTP 400, including equal duplicate values and conflicting case aliases. Validation also applies to supported multipart fields and Responses WebSocket frames, before routing or billing. Clients must serialize unambiguous objects. Normal fixed-console JSON requests continue to work. The model allowlist order, text body limits, composite rewrites and custom outbound address checks remain intact.

Configured platform quota defaults are snapshotted for newly created ordinary users, including administrator-created users; existing users are not backfilled. Snapshot errors remain best-effort and do not abort signup. The release includes updated Codex context types, tool search, Exa search backend support and streaming/buffered response compatibility. The fixed console, pricing-field passthrough and historical public pages remain unchanged; new official administration controls are not added. No mandatory production environment change is required; MCP and Ultra Fast remain disabled unless explicitly configured.

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

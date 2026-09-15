# Sub2API Custom Upgrade Workflow

This workspace contains the customized Sub2API app and the standalone leaderboard sidecar:

- App repository: `D:\Desktop\sub2api\sub2api-custom`
- Leaderboard repository: `D:\Desktop\sub2api\sub2api-leaderboard`
- Production deployment: `root@23.95.229.165:2222:/opt/sub2api-deploy`
- GitHub proxy: `http://127.0.0.1:10808`

The fixed deployment rule is: build the frontend, Linux binary, and Docker image locally. The production server only verifies the archive, loads the image, and runs it. Never run `pnpm build`, `go build`, or `docker build` on production.

Set the release values once:

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$oldTag = 'v0.1.158'
$newTag = 'v0.1.160'
$oldImage = 'sub2api-custom:0.1.158-ui1'
$newImage = 'sub2api-custom:0.1.160-ui1'
$backupSuffix = 'bak-v0160-ui1'
```

## 1. Fetch and Audit

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$env:HTTP_PROXY = 'http://127.0.0.1:10808'
$env:HTTPS_PROXY = 'http://127.0.0.1:10808'
$env:NO_PROXY = 'localhost,127.0.0.1'

Set-Location -LiteralPath 'D:\Desktop\sub2api\sub2api-custom'
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw "git status failed with exit code $LASTEXITCODE" }
& git fetch upstream --tags --prune
if ($LASTEXITCODE -ne 0) { throw "git fetch failed with exit code $LASTEXITCODE" }
& git diff --stat "$oldTag..$newTag"
if ($LASTEXITCODE -ne 0) { throw "git diff failed with exit code $LASTEXITCODE" }
```

Protect these custom boundaries during every merge:

- `backend/internal/config/config.go`: keep `https://pay.ldxp.cn` in CSP.
- `backend/internal/service/setting_public.go`: keep public menu filtering.
- `backend/internal/service/ops_homepage_status.go` and `ops_public_status.go`: keep public status APIs.
- `frontend/src/views/HomeView.vue`: keep the branded homepage and community modal.
- `frontend/src/views/auth/`: keep the default successful-login fallback at `/home`.
- `frontend/src/views/user/ExternalRechargeView.vue`: keep the protected recharge page.
- `frontend/public/logo.png` and `community-placeholder.jpg`: keep local brand assets.
- `backend/internal/web/*`: keep the customized embedded frontend behavior.
- `README.md`: keep the custom-edition README rather than restoring the upstream README.

## 2. Merge the Release

Only merge with a clean worktree:

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Set-Location -LiteralPath 'D:\Desktop\sub2api\sub2api-custom'
& git merge --no-ff v0.1.158 -m 'merge upstream v0.1.158 into custom build'
if ($LASTEXITCODE -ne 0) { throw "resolve merge conflicts before continuing" }
```

Upstream release tags can contain a stale `backend/cmd/server/VERSION`. Confirm it matches the release and correct it when necessary.

The leaderboard does not merge upstream app code. Recheck its contracts:

- `GET /api/v1/auth/me` with bearer authentication.
- `public.usage_logs` and `public.users`.
- Compose services `sub2api`, `postgres`, and network `sub2api-network`.
- Proxy routes `/custom/usage-leaderboard` and `/leaderboard/`.

## 3. Validate Locally

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Set-Location -LiteralPath 'D:\Desktop\sub2api\sub2api-custom'
& git diff --check
if ($LASTEXITCODE -ne 0) { throw "git diff check failed with exit code $LASTEXITCODE" }
& pnpm --dir frontend run test:run
if ($LASTEXITCODE -ne 0) { throw "frontend tests failed with exit code $LASTEXITCODE" }
& pnpm --dir frontend run typecheck
if ($LASTEXITCODE -ne 0) { throw "frontend typecheck failed with exit code $LASTEXITCODE" }
& pnpm --dir frontend run lint:check
if ($LASTEXITCODE -ne 0) { throw "frontend lint failed with exit code $LASTEXITCODE" }
& pnpm --dir frontend run build
if ($LASTEXITCODE -ne 0) { throw "frontend build failed with exit code $LASTEXITCODE" }

& go -C backend test -tags unit ./internal/service -run 'TestSettingService_GetPublicSettings_|TestOpsService_GetPublic'
if ($LASTEXITCODE -ne 0) { throw "backend service tests failed with exit code $LASTEXITCODE" }
& go -C backend test -tags embed ./internal/web
if ($LASTEXITCODE -ne 0) { throw "backend embed tests failed with exit code $LASTEXITCODE" }

Set-Location -LiteralPath 'D:\Desktop\sub2api\sub2api-leaderboard'
& go test ./...
if ($LASTEXITCODE -ne 0) { throw "leaderboard tests failed with exit code $LASTEXITCODE" }
```

## 4. Commit and Push

Push the validated custom merge directly to the fork's `main` branch:

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$env:HTTP_PROXY = 'http://127.0.0.1:10808'
$env:HTTPS_PROXY = 'http://127.0.0.1:10808'
$env:NO_PROXY = 'localhost,127.0.0.1'

Set-Location -LiteralPath 'D:\Desktop\sub2api\sub2api-custom'
& git add --all
if ($LASTEXITCODE -ne 0) { throw "git add failed with exit code $LASTEXITCODE" }
& git commit
if ($LASTEXITCODE -ne 0) { throw "git commit failed with exit code $LASTEXITCODE" }
& git push origin main
if ($LASTEXITCODE -ne 0) { throw "git push failed with exit code $LASTEXITCODE" }
```

Wait for both GitHub Actions workflows, `CI` and `Security Scan`, to succeed before production cleanup.

## 5. Build the Image Locally

The binary reports the upstream version; the Docker tag carries the custom build suffix.

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repo = 'D:\Desktop\sub2api\sub2api-custom'
$version = '0.1.160'
$image = 'sub2api-custom:0.1.160-ui1'
Set-Location -LiteralPath $repo

& pnpm --dir frontend run build
if ($LASTEXITCODE -ne 0) { throw "frontend build failed with exit code $LASTEXITCODE" }

$commit = (& git rev-parse --short=8 HEAD).Trim()
if ($LASTEXITCODE -ne 0) { throw "git rev-parse failed with exit code $LASTEXITCODE" }
$date = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
$binary = Join-Path $repo 'build-local\sub2api'
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $binary) | Out-Null

$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
& go -C backend build -tags embed -trimpath -ldflags "-s -w -X main.Version=$version -X main.Commit=$commit -X main.Date=$date -X main.BuildType=release" -o $binary ./cmd/server
if ($LASTEXITCODE -ne 0) { throw "backend build failed with exit code $LASTEXITCODE" }

& docker build --file Dockerfile.prebuilt-binary --tag $image .
if ($LASTEXITCODE -ne 0) {
  # Registry metadata may be unavailable even when the previous verified image is cached.
  & docker build --file Dockerfile.rebase-binary --build-arg 'RUNTIME_BASE_IMAGE=sub2api-custom:0.1.158-ui1' --tag $image .
  if ($LASTEXITCODE -ne 0) { throw "local Docker builds failed with exit code $LASTEXITCODE" }
}
& docker run --rm --entrypoint /app/sub2api $image -version
if ($LASTEXITCODE -ne 0) { throw "image version check failed with exit code $LASTEXITCODE" }
```

## 6. Export and Upload the Image

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$image = 'sub2api-custom:0.1.158-ui1'
$tag = '0.1.158-ui1'
$artifactRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('sub2api-deploy-' + $tag + '-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $artifactRoot | Out-Null
$imageTar = Join-Path $artifactRoot 'image.tar'
$archive = Join-Path $artifactRoot 'image.tar.gz'

& docker image save --output $imageTar $image
if ($LASTEXITCODE -ne 0) { throw "docker save failed with exit code $LASTEXITCODE" }
& tar -czf $archive -C $artifactRoot 'image.tar'
if ($LASTEXITCODE -ne 0) { throw "archive compression failed with exit code $LASTEXITCODE" }
$hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()

& ssh -p 2222 root@23.95.229.165 "mkdir -p /opt/sub2api-images/$tag"
if ($LASTEXITCODE -ne 0) { throw "remote directory creation failed with exit code $LASTEXITCODE" }
& scp -P 2222 $archive "root@23.95.229.165:/opt/sub2api-images/$tag/image.tar.gz"
if ($LASTEXITCODE -ne 0) { throw "image upload failed with exit code $LASTEXITCODE" }
& ssh -p 2222 root@23.95.229.165 "cd /opt/sub2api-images/$tag && echo '$hash  image.tar.gz' | sha256sum -c - && tar -xzf image.tar.gz"
if ($LASTEXITCODE -ne 0) { throw "remote archive verification failed with exit code $LASTEXITCODE" }
& ssh -p 2222 root@23.95.229.165 "docker load -i /opt/sub2api-images/$tag/image.tar"
if ($LASTEXITCODE -ne 0) { throw "remote docker load failed with exit code $LASTEXITCODE" }
```

## 7. Switch Only the App Container

Run this Bash script through the PowerShell SSH boundary. It does not stop PostgreSQL, Redis, or leaderboard.

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$remoteScript = @'
set -euo pipefail
cd /opt/sub2api-deploy
old_image='sub2api-custom:0.1.156-ui5'
new_image='sub2api-custom:0.1.158-ui1'
backup_suffix='bak-v0158-ui1'
timestamp="$(date +%Y%m%d-%H%M%S)"

docker image inspect "$new_image" >/dev/null
grep -Fq "image: $old_image" docker-compose.override.yml
cp docker-compose.override.yml "docker-compose.override.yml.${backup_suffix}-${timestamp}"
sed -i "s#image: $old_image#image: $new_image#" docker-compose.override.yml

docker compose -f docker-compose.local.yml -f docker-compose.override.yml -f docker-compose.leaderboard.yml up -d --no-deps sub2api
for attempt in $(seq 1 45); do
  health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}' sub2api 2>/dev/null || true)"
  echo "health=$health attempt=$attempt"
  [ "$health" = healthy ] && exit 0
  sleep 2
done
exit 1
'@

$remoteScript | & ssh -p 2222 root@23.95.229.165 "tr -d '\r' | bash -s"
if ($LASTEXITCODE -ne 0) { throw "production switch failed with exit code $LASTEXITCODE" }
```

If health fails, restore the timestamped Compose backup or replace the image with `sub2api-custom:0.1.156-ui5`, then rerun the same single-service Compose command.

## 8. Verify and Clean Up

Required checks:

- Container is `running | healthy | restart=0`.
- `https://api.zhouz.online/health` and `/home` return 200.
- The binary reports `Sub2API 0.1.158` and the deployed commit.
- Branded logo, community image, homepage asset, `/home` login fallback, and the account-menu GitHub removal are embedded.
- Recent logs contain no panic, fatal, or structured access `status_code: 5xx`.
- GitHub `CI` and `Security Scan` pass for the deployed commit.
- PostgreSQL, Redis, leaderboard, volumes, and deployment data remain intact.

Only after those checks:

- Delete `/opt/sub2api-images/0.1.158-ui1` after the image is loaded and verified.
- Delete application images older than the immediate rollback when disk pressure requires it.
- Keep current `sub2api-custom:0.1.158-ui1` and rollback `sub2api-custom:0.1.156-ui5`.
- Delete the local temporary artifact directory.
- Do not run a broad volume prune on production.

## 9. Notes for v0.1.158

- Upstream changes 388 files and adds audit logs, step-up 2FA for sensitive admin actions, batch user-limit updates, asynchronous image tasks, image-input billing, and upstream billing-rate probing.
- The release improves OpenAI/Grok scheduling, Responses field fallback, WebSocket handling, image routing, group duplication, and channel-monitor duplication.
- Direct custom overlaps are `.gitignore`, `backend/cmd/server/VERSION`, `backend/internal/config/config.go`, `backend/internal/server/router.go`, `backend/internal/service/openai_gateway_grok_test.go`, `frontend/src/components/layout/AppSidebar.vue`, the common locale files, `frontend/src/router/index.ts`, and `README.md`.
- Keep the custom payment CSP, homepage status services, embedded-frontend cache behavior, branded README, default `/home` login redirect, and community modal.
- The official `v0.1.158` tag still contains `backend/cmd/server/VERSION=0.1.157`; the custom build corrects it to `0.1.158`.
- `Dockerfile.rebase-binary` provides a local-only fallback when Docker Hub is unavailable by replacing the application layer on the previous verified runtime image.

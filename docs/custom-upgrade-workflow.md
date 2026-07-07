# Custom Upgrade Workflow

This workflow keeps the customized Sub2API app aligned with upstream releases while preserving the local custom menu, embedded page, and leaderboard sidecar contracts.

## Scope

- Main app repository: `D:\Desktop\sub2api\sub2api-custom`
- Leaderboard sidecar repository: `D:\Desktop\sub2api\sub2api-leaderboard`
- Production deployment path: `/opt/sub2api-deploy`

Set these values first, then keep every example aligned with them:

```powershell
$oldTag = 'v0.1.144'
$newTag = 'v0.1.146'
$oldImageTag = '0.1.144-ui1'
$newImageTag = '0.1.146-ui1'
$backupSuffix = 'bak-v0146'
```

## 1. Confirm the Upstream Tag

```powershell
cd D:\Desktop\sub2api\sub2api-custom
git -c http.proxy=http://127.0.0.1:10808 -c https.proxy=http://127.0.0.1:10808 ls-remote --tags upstream "$newTag"
git -c http.proxy=http://127.0.0.1:10808 -c https.proxy=http://127.0.0.1:10808 fetch upstream tag $newTag
git diff --stat "$oldTag..$newTag"
```

Replace tag names for future upgrades. Keep proxy flags only when direct GitHub access fails.

## 2. Audit Local Customizations

```powershell
git status --short
$trackedDirty = git diff --name-only
$staged = git diff --cached --name-only
$untracked = git ls-files --others --exclude-standard
$local = @($trackedDirty; $staged; $untracked) | Where-Object { $_ } | Sort-Object -Unique
$up = git diff --name-only "$oldTag..$newTag"
Compare-Object -ReferenceObject $local -DifferenceObject $up -IncludeEqual -ExcludeDifferent | ForEach-Object { $_.InputObject }
```

Review these recurring custom areas:

- `backend/internal/config/config.go`: CSP must keep `https://pay.ldxp.cn`.
- `backend/internal/service/setting_service.go`: upstream public settings and custom menu normalization must both remain.
- `frontend/src/components/layout/AppSidebar.vue`: `/recharge` and custom menu sidebar behavior must remain visible to users.
- `frontend/src/i18n/locales/en.ts` and `frontend/src/i18n/locales/zh.ts`: recharge text must remain valid.
- `frontend/src/views/user/CustomPageView.vue`: iframe token handling must remain.
- `backend/internal/web/*`: embedded frontend override must still serve the customized app shell.

## 3. Merge the Release

```powershell
git stash push -u -m "custom work before v0.1.146"
git merge --no-ff v0.1.146 -m "merge upstream v0.1.146 into custom build"
git stash apply 'stash@{0}'
```

Resolve conflicts by preserving both upstream release changes and local custom behavior. After verification, optionally drop the temporary stash:

```powershell
git stash list
git stash drop 'stash@{0}'
```

## 4. Check Leaderboard Compatibility

The leaderboard sidecar normally does not merge upstream Sub2API code. Recheck these contracts after every Sub2API upgrade:

- `SUB2API_BASE_URL`, default `http://sub2api:8080`.
- `GET /api/v1/auth/me` with `Authorization: Bearer <token>`.
- `public.usage_logs` and `public.users` columns used by `internal/store`.
- Compose service names `sub2api`, `postgres`, and `sub2api-network`.
- OpenResty overrides for `/custom/usage-leaderboard` and `/leaderboard/`.

For `v0.1.146`, no leaderboard Go code change was required beyond verifying the existing sidecar contracts.

## 5. Local Validation

```powershell
cd D:\Desktop\sub2api\sub2api-custom\backend
go test -tags unit ./internal/service -run "TestSettingService_GetPublicSettings_"
go test -tags embed ./internal/web

cd D:\Desktop\sub2api\sub2api-custom\frontend
pnpm test:run AppSidebar CustomPageView
pnpm typecheck

cd D:\Desktop\sub2api\sub2api-leaderboard
go test ./...
```

Validate the compose overlay in a temporary production-like layout:

```powershell
$checkRoot = Join-Path $env:TEMP ('sub2api-compose-check-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $checkRoot | Out-Null
New-Item -ItemType Directory -Path (Join-Path $checkRoot 'leaderboard') | Out-Null
Copy-Item -LiteralPath 'D:\Desktop\sub2api\sub2api-custom\deploy\docker-compose.local.yml' -Destination (Join-Path $checkRoot 'docker-compose.local.yml')
Copy-Item -LiteralPath 'D:\Desktop\sub2api\sub2api-leaderboard\deploy\docker-compose.leaderboard.yml' -Destination (Join-Path $checkRoot 'docker-compose.leaderboard.yml')
New-Item -ItemType File -Path (Join-Path $checkRoot 'leaderboard\.env') | Out-Null
$env:POSTGRES_PASSWORD='dummy-compose-check'
$env:REDIS_PASSWORD='dummy-compose-check'
docker compose --project-directory $checkRoot -f (Join-Path $checkRoot 'docker-compose.local.yml') -f (Join-Path $checkRoot 'docker-compose.leaderboard.yml') config --quiet
$composeExit = $LASTEXITCODE
Remove-Item -LiteralPath $checkRoot -Recurse -Force
exit $composeExit
```

## 6. Publish to GitHub

```powershell
cd D:\Desktop\sub2api\sub2api-custom
git status -sb
git add <intended files>
git commit -m "chore: update custom build for v0.1.146"
git push -u origin codex/fix-custom-menu-injection

cd D:\Desktop\sub2api\sub2api-leaderboard
git status -sb
git add <intended files>
git commit -m "chore: prepare leaderboard deployment refresh"
git push -u origin codex/usage-leaderboard
```

If the leaderboard repository has no remote yet, create a private GitHub repository:

```powershell
gh repo create juanmao1z/sub2api-leaderboard --private --source . --remote origin --push
```

## 7. Build Custom Runtime Image

```powershell
cd D:\Desktop\sub2api\sub2api-custom
pnpm --dir frontend run build

cd D:\Desktop\sub2api\sub2api-custom\backend
$tag = '0.1.146-ui1'
New-Item -ItemType Directory -Force -Path '..\build-local' | Out-Null
$commit = git -C .. rev-parse --short HEAD
$date = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
$env:CGO_ENABLED='0'
$env:GOOS='linux'
$env:GOARCH='amd64'
go build -tags embed -trimpath -ldflags "-s -w -X main.Version=$tag -X main.Commit=$commit -X main.Date=$date -X main.BuildType=release" -o ..\build-local\sub2api .\cmd\server
```

```powershell
cd D:\Desktop\sub2api\sub2api-custom
$tag = '0.1.146-ui1'
$artifactRoot = Join-Path (Get-Location) 'deploy-artifacts'
$contextRoot = Join-Path $artifactRoot "sub2api-custom-$tag-context"
$archive = Join-Path $artifactRoot "sub2api-custom-$tag-context.tar.gz"
if (Test-Path -LiteralPath $contextRoot) { Remove-Item -LiteralPath $contextRoot -Recurse -Force }
New-Item -ItemType Directory -Force -Path (Join-Path $contextRoot 'build-local') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $contextRoot 'backend') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $contextRoot 'deploy') | Out-Null
Copy-Item -LiteralPath 'Dockerfile.prebuilt-binary' -Destination (Join-Path $contextRoot 'Dockerfile')
Copy-Item -LiteralPath 'build-local\sub2api' -Destination (Join-Path $contextRoot 'build-local\sub2api')
Copy-Item -LiteralPath 'backend\resources' -Destination (Join-Path $contextRoot 'backend\resources') -Recurse
Copy-Item -LiteralPath 'deploy\docker-entrypoint.sh' -Destination (Join-Path $contextRoot 'deploy\docker-entrypoint.sh')
if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive -Force }
tar -czf $archive -C $contextRoot .
ssh root@23.95.229.165 "mkdir -p /opt/sub2api-build/sub2api-custom-$tag"
scp $archive root@23.95.229.165:/opt/sub2api-build/sub2api-custom-$tag/context.tar.gz
```

```bash
tag=0.1.146-ui1
cd /opt/sub2api-build/sub2api-custom-$tag
rm -rf context
mkdir context
tar -xzf context.tar.gz -C context
docker build -t sub2api-custom:$tag context
```

## 8. Deploy on Server

```bash
cd /opt/sub2api-deploy
old_tag=0.1.144-ui1
new_tag=0.1.146-ui1
backup_suffix=bak-v0146
ts=$(date +%Y%m%d-%H%M%S)
cp docker-compose.override.yml docker-compose.override.yml.$backup_suffix-$ts
sed -i "s#sub2api-custom:${old_tag}#sub2api-custom:${new_tag}#" docker-compose.override.yml
docker compose -f docker-compose.local.yml -f docker-compose.override.yml -f docker-compose.leaderboard.yml up -d sub2api
```

```bash
for i in $(seq 1 30); do
  status=$(docker inspect -f "{{.State.Health.Status}}" sub2api 2>/dev/null || echo missing)
  echo "sub2api health=$status"
  [ "$status" = healthy ] && exit 0
  sleep 2
done
exit 1
```

## 9. Server Validation

```bash
cd /opt/sub2api-deploy
docker compose -f docker-compose.local.yml -f docker-compose.override.yml -f docker-compose.leaderboard.yml ps
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8095/leaderboard/health
docker exec sub2api-postgres psql -U sub2api -d sub2api -c "\d public.usage_logs"
docker exec sub2api-postgres psql -U sub2api -d sub2api -c "\d public.users"
```

Then verify `/custom/usage-leaderboard` in the browser. The iframe should load, and the visible URL should not retain the user token.

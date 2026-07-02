# Custom Upgrade Workflow

This workflow keeps the customized Sub2API app aligned with upstream releases while preserving the local custom menu, embedded page, and leaderboard sidecar contracts.

## Scope

- Main app repository: `D:\Desktop\sub2api\sub2api-custom`
- Leaderboard sidecar repository: `D:\Desktop\sub2api\sub2api-leaderboard`
- Production deployment path: `/opt/sub2api-deploy`

## 1. Confirm the Upstream Tag

```powershell
cd D:\Desktop\sub2api\sub2api-custom
git -c http.proxy=http://127.0.0.1:10808 -c https.proxy=http://127.0.0.1:10808 ls-remote --tags upstream "v0.1.*"
git -c http.proxy=http://127.0.0.1:10808 -c https.proxy=http://127.0.0.1:10808 fetch upstream tag v0.1.143
git diff --stat v0.1.142..v0.1.143
```

Replace tag names for future upgrades. Keep proxy flags only when direct GitHub access fails.

## 2. Audit Local Customizations

```powershell
git status --short
$local = git diff --name-only
$up = git diff --name-only v0.1.142..v0.1.143
Compare-Object -ReferenceObject $local -DifferenceObject $up -IncludeEqual -ExcludeDifferent | ForEach-Object { $_.InputObject }
```

Review these recurring custom areas:

- `backend/internal/config/config.go`: CSP must keep `https://pay.ldxp.cn`.
- `backend/internal/service/setting_service.go`: upstream public settings and custom menu normalization must both remain.
- `frontend/src/i18n/locales/en.ts` and `frontend/src/i18n/locales/zh.ts`: recharge text must remain valid.
- `frontend/src/views/user/CustomPageView.vue`: iframe token handling must remain.
- `backend/internal/web/*`: embedded frontend override must still serve the customized app shell.

## 3. Merge the Release

```powershell
git stash push -u -m "custom work before v0.1.143"
git merge --no-ff v0.1.143 -m "merge upstream v0.1.143 into custom build"
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

For `v0.1.143`, no leaderboard Go code change was required.

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

## 6. Server Validation

```bash
cd /opt/sub2api-deploy
docker compose -f docker-compose.local.yml -f docker-compose.leaderboard.yml ps
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8095/leaderboard/health
docker exec sub2api-postgres psql -U sub2api -d sub2api -c "\d public.usage_logs"
docker exec sub2api-postgres psql -U sub2api -d sub2api -c "\d public.users"
```

Then verify `/custom/usage-leaderboard` in the browser. The iframe should load, and the visible URL should not retain the user token.

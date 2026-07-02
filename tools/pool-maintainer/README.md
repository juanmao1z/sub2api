# Pool Maintainer

Windows local helper for semi-automatic Sub2API account pool maintenance.

## Workflow

1. Copy `pool-maintainer.example.yaml` to `pool-maintainer.yaml`.
2. Fill in real local group IDs and upstream account name patterns.
3. Open upstream pricing pages:

```powershell
go run ./cmd/pool-maintainer open-browser --config ..\tools\pool-maintainer\pool-maintainer.yaml --profiles-dir ..\runs\pool-maintainer-profiles
```

4. Log in to each upstream site in the opened browser, then save each pricing page HTML as `<upstream_id>.html` in a snapshot directory, for example `runs\pool-maintainer-snapshots\mdkj.html`.
5. Generate the review files:

```powershell
$env:SUB2API_ADMIN_TOKEN = "your-admin-token"
go run ./cmd/pool-maintainer collect --config ..\tools\pool-maintainer\pool-maintainer.yaml --html-dir ..\runs\pool-maintainer-snapshots --out ..\runs\pool-maintainer-20260703
```

6. Review `report.html` and `apply-plan.json`.
7. Dry-run or apply the approved plan:

```powershell
go run ./cmd/pool-maintainer apply --config ..\tools\pool-maintainer\pool-maintainer.yaml --plan ..\runs\pool-maintainer-20260703\apply-plan.json --dry-run
go run ./cmd/pool-maintainer apply --config ..\tools\pool-maintainer\pool-maintainer.yaml --plan ..\runs\pool-maintainer-20260703\apply-plan.json
```

## Safety Notes

- The tool never writes directly to the database.
- `collect` reads account state through Admin API and writes only local report files.
- `apply` checks account drift before calling Admin API.
- Admin tokens are read only from the configured environment variable.
- If upstream collection fails, the related accounts remain unchanged and are marked in red in the report.

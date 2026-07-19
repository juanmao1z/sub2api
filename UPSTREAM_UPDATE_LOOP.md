# Upstream update loop

`tools/update-upstream-loop.ps1` turns the manual upstream merge process into a bounded local loop.
It fetches official tags, creates an isolated worktree, merges the latest stable tag without
committing, validates protected custom behavior, and permits at most two Codex repair attempts.

The loop never commits, pushes, or deploys. A successful run stops with an uncommitted merge and a
report under `.git/update-loop/` for manual review.

## Preview

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Set-Location -LiteralPath 'D:\Desktop\sub2api\sub2api-custom'
& pwsh -NoLogo -NoProfile -File '.\tools\update-upstream-loop.ps1' -PlanOnly
if ($LASTEXITCODE -ne 0) { throw "Update loop preview failed: $LASTEXITCODE" }
```

## Run

The source `main` worktree must be clean. The script verifies that the configured Windows system
proxy is enabled and that `127.0.0.1:10808` is accepting connections before it contacts GitHub.

```powershell
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Set-Location -LiteralPath 'D:\Desktop\sub2api\sub2api-custom'
& pwsh -NoLogo -NoProfile -File '.\tools\update-upstream-loop.ps1'
if ($LASTEXITCODE -ne 0) { throw "Update loop failed: $LASTEXITCODE" }
```

Use `-TargetTag v0.1.162` to select a specific official stable tag. Use `-NoAiRepair` when only a
deterministic merge and validation report is wanted. `-SkipFetch` exists for offline diagnostics;
it must not be used to claim that the local tag set is the latest official release.

## Resume and review

Rerun the same command to resume an interrupted update. State, logs, and the report are stored in
`.git/update-loop/`; update worktrees are stored in `.worktrees/update-<tag>`.

At the approval boundary, inspect the report and worktree. Do not commit until the diff and every
validation result have been reviewed. Build, push, and production deployment remain governed by
the separate deployment workflow.

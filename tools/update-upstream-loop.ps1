[CmdletBinding()]
param(
    [string]$RepoPath = 'D:\Desktop\sub2api\sub2api-custom',
    [string]$ConfigPath = (Join-Path $PSScriptRoot 'update-loop.config.json'),
    [string]$TargetTag,
    [switch]$PlanOnly,
    [switch]$SkipFetch,
    [switch]$NoAiRepair
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Invoke-NativeCommand {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Executable,

        [string[]]$Arguments = @(),

        [Parameter(Mandatory)]
        [string]$WorkingDirectory,

        [int[]]$AllowedExitCodes = @(0),

        [string]$LogPath,

        [switch]$Quiet,

        [AllowNull()]
        [string]$StandardInput
    )

    $resolvedWorkingDirectory = (Resolve-Path -LiteralPath $WorkingDirectory).Path
    Push-Location -LiteralPath $resolvedWorkingDirectory
    try {
        if ($PSBoundParameters.ContainsKey('StandardInput')) {
            $rawOutput = $StandardInput | & $Executable @Arguments 2>&1
            $exitCode = $LASTEXITCODE
        } else {
            $rawOutput = & $Executable @Arguments 2>&1
            $exitCode = $LASTEXITCODE
        }
    } finally {
        Pop-Location
    }

    $output = @($rawOutput | ForEach-Object { [string]$_ })
    if ($LogPath) {
        $logDirectory = Split-Path -Parent $LogPath
        New-Item -ItemType Directory -Path $logDirectory -Force | Out-Null
        $output | Set-Content -LiteralPath $LogPath -Encoding UTF8
    }
    if (-not $Quiet) {
        foreach ($line in $output) {
            Write-Host $line
        }
    }

    if ($exitCode -notin $AllowedExitCodes) {
        throw "$Executable failed with exit code $exitCode."
    }

    return [pscustomobject]@{
        ExitCode = $exitCode
        Output = $output
    }
}

function Invoke-Git {
    param(
        [Parameter(Mandatory)]
        [string]$Repository,

        [Parameter(Mandatory)]
        [string[]]$Arguments,

        [int[]]$AllowedExitCodes = @(0),

        [string]$LogPath
    )

    $gitArguments = @('-C', $Repository) + $Arguments
    return Invoke-NativeCommand -Executable 'git' -Arguments $gitArguments `
        -WorkingDirectory $Repository -AllowedExitCodes $AllowedExitCodes -LogPath $LogPath -Quiet
}

function Test-GitHubProxy {
    param(
        [Parameter(Mandatory)]
        [uri]$ProxyUri
    )

    $internetSettings = Get-ItemProperty `
        -LiteralPath 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings'
    $expectedEndpoint = "$($ProxyUri.Host):$($ProxyUri.Port)"
    if ([int]$internetSettings.ProxyEnable -ne 1) {
        throw "Windows system proxy is disabled; expected $expectedEndpoint."
    }
    if ([string]$internetSettings.ProxyServer -notlike "*$expectedEndpoint*") {
        throw "Windows system proxy is '$($internetSettings.ProxyServer)'; expected $expectedEndpoint."
    }

    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $task = $client.ConnectAsync($ProxyUri.Host, $ProxyUri.Port)
        if (-not $task.Wait([TimeSpan]::FromSeconds(2)) -or -not $client.Connected) {
            throw "Proxy endpoint is not accepting connections: $expectedEndpoint."
        }
    } finally {
        $client.Dispose()
    }
}

function ConvertTo-VersionedTag {
    param(
        [Parameter(Mandatory)]
        [string]$Tag,

        [Parameter(Mandatory)]
        [string]$Pattern
    )

    if ($Tag -notmatch $Pattern) {
        return $null
    }
    return [pscustomobject]@{
        Tag = $Tag
        Version = [version]$Tag.Substring(1)
    }
}

function Get-VersionedTags {
    param(
        [Parameter(Mandatory)]
        [string]$Repository,

        [Parameter(Mandatory)]
        [string]$Pattern,

        [string]$ReachableFrom
    )

    $tagResult = Invoke-Git -Repository $Repository -Arguments @('tag', '--list')
    $tags = [System.Collections.Generic.List[object]]::new()
    foreach ($tag in $tagResult.Output) {
        $candidate = ConvertTo-VersionedTag -Tag $tag.Trim() -Pattern $Pattern
        if ($null -eq $candidate) {
            continue
        }
        if ($ReachableFrom) {
            $ancestor = Invoke-Git -Repository $Repository `
                -Arguments @('merge-base', '--is-ancestor', $candidate.Tag, $ReachableFrom) `
                -AllowedExitCodes @(0, 1)
            if ($ancestor.ExitCode -eq 1) {
                continue
            }
        }
        $tags.Add($candidate)
    }
    return @($tags | Sort-Object Version -Descending)
}

function Save-LoopState {
    param(
        [Parameter(Mandatory)]
        [object]$State,

        [Parameter(Mandatory)]
        [string]$Path
    )

    $State.updatedAt = (Get-Date).ToUniversalTime().ToString('o')
    $temporaryPath = "$Path.tmp"
    $State | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $temporaryPath -Encoding UTF8
    Move-Item -LiteralPath $temporaryPath -Destination $Path -Force
}

function Get-ConflictedPaths {
    param(
        [Parameter(Mandatory)]
        [string]$Repository
    )

    $result = Invoke-Git -Repository $Repository -Arguments @('diff', '--name-only', '--diff-filter=U')
    return @($result.Output | Where-Object { $_.Trim().Length -gt 0 })
}

function Add-ResolvedConflictPaths {
    param(
        [Parameter(Mandatory)]
        [string]$Repository
    )

    $conflicts = Get-ConflictedPaths -Repository $Repository
    if ($conflicts.Count -eq 0) {
        return @()
    }

    $checkArguments = @('diff', '--check', '--') + $conflicts
    $check = Invoke-Git -Repository $Repository -Arguments $checkArguments `
        -AllowedExitCodes @(0, 1, 2)
    if ($check.ExitCode -ne 0) {
        return $conflicts
    }

    $addArguments = @('add', '-A', '--') + $conflicts
    Invoke-Git -Repository $Repository -Arguments $addArguments | Out-Null
    return Get-ConflictedPaths -Repository $Repository
}

function Test-MergeInProgress {
    param(
        [Parameter(Mandatory)]
        [string]$Repository
    )

    $result = Invoke-Git -Repository $Repository `
        -Arguments @('rev-parse', '--quiet', '--verify', 'MERGE_HEAD') -AllowedExitCodes @(0, 1)
    return $result.ExitCode -eq 0
}

function Get-FailureContext {
    param(
        [Parameter(Mandatory)]
        [string[]]$LogPaths
    )

    $sections = [System.Collections.Generic.List[string]]::new()
    foreach ($logPath in $LogPaths) {
        if (-not (Test-Path -LiteralPath $logPath -PathType Leaf)) {
            continue
        }
        $tail = Get-Content -LiteralPath $logPath -Tail 200 -Encoding UTF8
        $sections.Add("### $(Split-Path -Leaf $logPath)`n$($tail -join "`n")")
    }
    return $sections -join "`n`n"
}

function Invoke-AiRepair {
    param(
        [Parameter(Mandatory)]
        [string]$Worktree,

        [Parameter(Mandatory)]
        [string]$TargetVersionTag,

        [Parameter(Mandatory)]
        [object]$Config,

        [Parameter(Mandatory)]
        [string]$FailureContext,

        [Parameter(Mandatory)]
        [string]$LogDirectory,

        [Parameter(Mandatory)]
        [int]$Attempt
    )

    $boundarySummary = @($Config.protectedChecks | ForEach-Object {
        "- $($_.path): $($_.contains -join '; ')"
    }) -join "`n"
    $prompt = @"
You are repairing an in-progress merge of official Sub2API $TargetVersionTag into a custom fork.

Work only inside the current worktree. Resolve the reported conflicts or validation failures with
the smallest possible patch. Preserve upstream behavior except where the listed custom boundaries
require custom behavior. Do not fetch, pull, push, commit, merge, rebase, reset, checkout, deploy,
stage files, or modify Git metadata. Do not weaken or delete tests. Do not add unrelated features
or refactors. The outer controller will stage only conflict paths after checking them.

Protected custom boundaries:
$boundarySummary

Failure context:
$FailureContext

Finish by summarizing files changed and checks you ran. The outer controller will independently
run all required validations and decide whether the repair succeeded.
"@

    $lastMessagePath = Join-Path $LogDirectory "ai-repair-$Attempt-final.txt"
    $eventLogPath = Join-Path $LogDirectory "ai-repair-$Attempt.log"
    $headBefore = (Invoke-Git -Repository $Worktree -Arguments @('rev-parse', 'HEAD')).Output[0].Trim()
    $result = Invoke-NativeCommand -Executable 'codex' `
        -Arguments @(
            'exec',
            '--ephemeral',
            '--sandbox', 'workspace-write',
            '--cd', $Worktree,
            '--output-last-message', $lastMessagePath,
            '-'
        ) `
        -WorkingDirectory $Worktree -AllowedExitCodes @(0, 1) -LogPath $eventLogPath `
        -StandardInput $prompt
    $headAfter = (Invoke-Git -Repository $Worktree -Arguments @('rev-parse', 'HEAD')).Output[0].Trim()
    if ($headAfter -ne $headBefore) {
        throw 'AI repair changed HEAD. Stop and inspect the worktree; no automatic reset was performed.'
    }
    if ($result.ExitCode -ne 0) {
        Write-Warning "Codex repair attempt $Attempt exited with code $($result.ExitCode)."
    }
}

function Invoke-ValidationSuite {
    param(
        [Parameter(Mandatory)]
        [string]$Worktree,

        [Parameter(Mandatory)]
        [string]$TargetVersionTag,

        [Parameter(Mandatory)]
        [object]$Config,

        [Parameter(Mandatory)]
        [string]$BoundaryScript,

        [Parameter(Mandatory)]
        [string]$ConfigFile,

        [Parameter(Mandatory)]
        [string]$LogDirectory,

        [Parameter(Mandatory)]
        [int]$Round
    )

    $results = [System.Collections.Generic.List[object]]::new()
    $boundaryLog = Join-Path $LogDirectory "validation-$Round-custom-boundaries.log"
    try {
        $boundaryResult = Invoke-NativeCommand -Executable (Join-Path $PSHOME 'pwsh.exe') `
            -Arguments @(
                '-NoLogo', '-NoProfile', '-File', $BoundaryScript,
                '-RepoPath', $Worktree,
                '-ConfigPath', $ConfigFile,
                '-TargetTag', $TargetVersionTag
            ) -WorkingDirectory $Worktree -AllowedExitCodes @(0, 1) -LogPath $boundaryLog
        $results.Add([pscustomobject]@{
            Name = 'custom-boundaries'
            ExitCode = $boundaryResult.ExitCode
            LogPath = $boundaryLog
        })
    } catch {
        $_.Exception.Message | Set-Content -LiteralPath $boundaryLog -Encoding UTF8
        $results.Add([pscustomobject]@{
            Name = 'custom-boundaries'
            ExitCode = 1
            LogPath = $boundaryLog
        })
    }

    foreach ($step in $Config.validationSteps) {
        $workingDirectory = if ([System.IO.Path]::IsPathRooted([string]$step.workingDirectory)) {
            [string]$step.workingDirectory
        } else {
            Join-Path $Worktree ([string]$step.workingDirectory)
        }
        $logPath = Join-Path $LogDirectory "validation-$Round-$($step.name).log"
        try {
            $commandResult = Invoke-NativeCommand -Executable ([string]$step.executable) `
                -Arguments @($step.arguments | ForEach-Object { [string]$_ }) `
                -WorkingDirectory $workingDirectory -AllowedExitCodes @(0, 1) -LogPath $logPath
            $exitCode = $commandResult.ExitCode
        } catch {
            $_.Exception.Message | Set-Content -LiteralPath $logPath -Encoding UTF8
            $exitCode = 1
        }
        $results.Add([pscustomobject]@{
            Name = [string]$step.name
            ExitCode = $exitCode
            LogPath = $logPath
        })
    }

    return [pscustomobject]@{
        Passed = @($results | Where-Object { $_.ExitCode -ne 0 }).Count -eq 0
        Results = @($results)
        FailedLogs = @($results | Where-Object { $_.ExitCode -ne 0 } | ForEach-Object { $_.LogPath })
    }
}

function Write-UpgradeReport {
    param(
        [Parameter(Mandatory)]
        [object]$State,

        [Parameter(Mandatory)]
        [string]$Repository,

        [Parameter(Mandatory)]
        [string]$Path
    )

    $status = (Invoke-Git -Repository $Repository -Arguments @('status', '--short', '--branch')).Output
    $stat = (Invoke-Git -Repository $Repository -Arguments @('diff', 'HEAD', '--stat')).Output
    $changed = (Invoke-Git -Repository $Repository -Arguments @('diff', 'HEAD', '--name-status')).Output
    $validationLines = @($State.validation | ForEach-Object {
        "- $($_.Name): $(if ($_.ExitCode -eq 0) { 'PASS' } else { 'FAIL' }) ($($_.LogPath))"
    })
    $report = @"
# Sub2API upstream update report

- Status: $($State.phase)
- Source tag: $($State.currentTag)
- Target tag: $($State.targetTag)
- Branch: $($State.branch)
- Worktree: $($State.worktree)
- AI repair attempts: $($State.repairAttempts)
- Started: $($State.startedAt)
- Updated: $($State.updatedAt)

## Git status

``````text
$($status -join "`n")
``````

## Diff stat

``````text
$($stat -join "`n")
``````

## Changed files

``````text
$($changed -join "`n")
``````

## Validation

$($validationLines -join "`n")

## Approval boundary

The loop did not commit, push, or deploy. Review this worktree and its logs. Commit only after
manual approval. Production build and deployment remain separate operations.
"@
    $report | Set-Content -LiteralPath $Path -Encoding UTF8
}

$repo = (Resolve-Path -LiteralPath $RepoPath).Path
$configFile = (Resolve-Path -LiteralPath $ConfigPath).Path
$config = Get-Content -LiteralPath $configFile -Raw -Encoding UTF8 | ConvertFrom-Json
if ($config.schemaVersion -ne 1) {
    throw "Unsupported update loop config schema: $($config.schemaVersion)"
}

$branchResult = Invoke-Git -Repository $repo -Arguments @('branch', '--show-current')
$currentBranch = ($branchResult.Output -join '').Trim()
if ($currentBranch -ne $config.sourceBranch) {
    throw "Expected branch '$($config.sourceBranch)', but '$currentBranch' is checked out."
}

$statusResult = Invoke-Git -Repository $repo -Arguments @('status', '--porcelain')
$isClean = @($statusResult.Output | Where-Object { $_.Trim().Length -gt 0 }).Count -eq 0
if (-not $PlanOnly -and -not $isClean) {
    throw 'The source worktree is not clean. Preserve or commit existing changes before running the loop.'
}

$upstreamUrlResult = Invoke-Git -Repository $repo `
    -Arguments @('remote', 'get-url', [string]$config.upstreamRemote)
$actualUpstreamUrl = ($upstreamUrlResult.Output -join '').Trim().TrimEnd('/')
$expectedUpstreamUrl = ([string]$config.upstreamUrl).Trim().TrimEnd('/')
if ($actualUpstreamUrl -ne $expectedUpstreamUrl) {
    throw "Remote '$($config.upstreamRemote)' is '$actualUpstreamUrl'; expected '$expectedUpstreamUrl'."
}

if (-not $SkipFetch) {
    $proxyUri = [uri]$config.proxyUrl
    Test-GitHubProxy -ProxyUri $proxyUri
    $env:HTTP_PROXY = $config.proxyUrl
    $env:HTTPS_PROXY = $config.proxyUrl
    $env:NO_PROXY = 'localhost,127.0.0.1'
    $upstreamRefSpec = "+refs/heads/$($config.upstreamBranch):refs/remotes/$($config.upstreamRemote)/$($config.upstreamBranch)"
    Invoke-Git -Repository $repo `
        -Arguments @('fetch', [string]$config.upstreamRemote, '--tags', '--prune', $upstreamRefSpec) | Out-Null
}

$currentTags = Get-VersionedTags -Repository $repo -Pattern $config.tagPattern `
    -ReachableFrom ([string]$config.sourceBranch)
if ($currentTags.Count -eq 0) {
    throw "No version tag is reachable from $($config.sourceBranch)."
}
$currentTag = $currentTags[0]

if ($TargetTag) {
    $targetCandidate = ConvertTo-VersionedTag -Tag $TargetTag -Pattern $config.tagPattern
    if ($null -eq $targetCandidate) {
        throw "Target tag does not match $($config.tagPattern): $TargetTag"
    }
    $targetExists = Invoke-Git -Repository $repo -Arguments @('show-ref', '--verify', '--quiet', "refs/tags/$TargetTag") `
        -AllowedExitCodes @(0, 1)
    if ($targetExists.ExitCode -ne 0) {
        throw "Target tag does not exist locally: $TargetTag"
    }
    $onUpstream = Invoke-Git -Repository $repo `
        -Arguments @('merge-base', '--is-ancestor', $TargetTag, [string]$config.upstreamMainRef) `
        -AllowedExitCodes @(0, 1)
    if ($onUpstream.ExitCode -ne 0) {
        throw "Target tag is not reachable from $($config.upstreamMainRef): $TargetTag"
    }
    $targetTagObject = $targetCandidate
} else {
    $upstreamTags = Get-VersionedTags -Repository $repo -Pattern $config.tagPattern `
        -ReachableFrom ([string]$config.upstreamMainRef)
    if ($upstreamTags.Count -eq 0) {
        throw "No version tag is reachable from $($config.upstreamMainRef)."
    }
    $targetTagObject = $upstreamTags[0]
}

Write-Host "Current custom baseline: $($currentTag.Tag)"
Write-Host "Latest selected upstream tag: $($targetTagObject.Tag)"
Write-Host "Source worktree clean: $isClean"

if ($PlanOnly) {
    Write-Host 'Plan-only mode completed; no worktree, branch, merge, commit, push, or deployment was created.'
    return
}

if ($targetTagObject.Version -le $currentTag.Version) {
    Write-Host 'No update is required.'
    return
}

$commonDirectoryResult = Invoke-Git -Repository $repo -Arguments @('rev-parse', '--git-common-dir')
$commonDirectoryValue = ($commonDirectoryResult.Output -join '').Trim()
$commonDirectory = if ([System.IO.Path]::IsPathRooted($commonDirectoryValue)) {
    [System.IO.Path]::GetFullPath($commonDirectoryValue)
} else {
    [System.IO.Path]::GetFullPath((Join-Path $repo $commonDirectoryValue))
}
$loopRoot = Join-Path $commonDirectory 'update-loop'
New-Item -ItemType Directory -Path $loopRoot -Force | Out-Null
$lockPath = Join-Path $loopRoot 'run.lock'
$lockStream = $null

try {
    try {
        $lockStream = [System.IO.File]::Open(
            $lockPath,
            [System.IO.FileMode]::OpenOrCreate,
            [System.IO.FileAccess]::ReadWrite,
            [System.IO.FileShare]::None
        )
    } catch {
        throw "Another update loop is running or left the lock open: $lockPath"
    }

    $safeTag = $targetTagObject.Tag -replace '[^0-9A-Za-z._-]', '-'
    $upgradeBranch = "update/$safeTag"
    $worktree = Join-Path $repo ".worktrees\update-$safeTag"
    $statePath = Join-Path $loopRoot "state-$safeTag.json"
    $logDirectory = Join-Path $loopRoot "logs-$safeTag"
    $reportPath = Join-Path $loopRoot "report-$safeTag.md"
    New-Item -ItemType Directory -Path $logDirectory -Force | Out-Null

    if (Test-Path -LiteralPath $statePath -PathType Leaf) {
        $state = Get-Content -LiteralPath $statePath -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($state.targetTag -ne $targetTagObject.Tag -or $state.worktree -ne $worktree) {
            throw "Existing state does not match this run: $statePath"
        }
    } else {
        $branchExists = Invoke-Git -Repository $repo `
            -Arguments @('show-ref', '--verify', '--quiet', "refs/heads/$upgradeBranch") `
            -AllowedExitCodes @(0, 1)
        if ($branchExists.ExitCode -eq 0 -or (Test-Path -LiteralPath $worktree)) {
            throw "Upgrade branch or worktree already exists without state: $upgradeBranch / $worktree"
        }
        $state = [pscustomobject][ordered]@{
            schemaVersion = 1
            phase = 'initializing'
            currentTag = $currentTag.Tag
            targetTag = $targetTagObject.Tag
            branch = $upgradeBranch
            worktree = $worktree
            repairAttempts = 0
            validationRound = 0
            validation = @()
            startedAt = (Get-Date).ToUniversalTime().ToString('o')
            updatedAt = (Get-Date).ToUniversalTime().ToString('o')
        }
        Save-LoopState -State $state -Path $statePath
        Invoke-Git -Repository $repo `
            -Arguments @('worktree', 'add', '-b', $upgradeBranch, $worktree, [string]$config.sourceBranch) | Out-Null
        $state.phase = 'worktree_created'
        Save-LoopState -State $state -Path $statePath
    }

    if (-not (Test-Path -LiteralPath $worktree -PathType Container)) {
        throw "State worktree is missing: $worktree"
    }
    if ($state.phase -eq 'awaiting_approval') {
        Write-Host "This update is already awaiting approval: $reportPath"
        return
    }

    $mergeInProgress = Test-MergeInProgress -Repository $worktree
    $integrated = Invoke-Git -Repository $worktree `
        -Arguments @('merge-base', '--is-ancestor', $targetTagObject.Tag, 'HEAD') `
        -AllowedExitCodes @(0, 1)
    if (-not $mergeInProgress -and $integrated.ExitCode -ne 0) {
        $mergeLog = Join-Path $logDirectory 'merge.log'
        $mergeResult = Invoke-Git -Repository $worktree `
            -Arguments @('merge', '--no-ff', '--no-commit', $targetTagObject.Tag) `
            -AllowedExitCodes @(0, 1) -LogPath $mergeLog
        if ($mergeResult.ExitCode -ne 0 -and (Get-ConflictedPaths -Repository $worktree).Count -eq 0) {
            throw "Merge failed without resolvable file conflicts. See $mergeLog"
        }
        $state.phase = if ($mergeResult.ExitCode -eq 0) { 'merged_uncommitted' } else { 'merge_conflicts' }
        Save-LoopState -State $state -Path $statePath
    }

    $conflicts = Get-ConflictedPaths -Repository $worktree
    while ($conflicts.Count -gt 0 -and $state.repairAttempts -lt $config.maxRepairAttempts -and -not $NoAiRepair) {
        $state.repairAttempts = [int]$state.repairAttempts + 1
        $failureContext = "Unresolved merge conflicts:`n$($conflicts -join "`n")"
        Invoke-AiRepair -Worktree $worktree -TargetVersionTag $targetTagObject.Tag `
            -Config $config -FailureContext $failureContext -LogDirectory $logDirectory `
            -Attempt $state.repairAttempts
        $conflicts = Add-ResolvedConflictPaths -Repository $worktree
        Save-LoopState -State $state -Path $statePath
    }
    if ($conflicts.Count -gt 0) {
        $state.phase = 'failed_conflicts'
        Save-LoopState -State $state -Path $statePath
        throw "Unresolved conflicts remain in ${worktree}: $($conflicts -join ', ')"
    }

    while ($true) {
        $state.validationRound = [int]$state.validationRound + 1
        $validation = Invoke-ValidationSuite -Worktree $worktree `
            -TargetVersionTag $targetTagObject.Tag -Config $config `
            -BoundaryScript (Join-Path $PSScriptRoot 'Test-CustomBoundaries.ps1') `
            -ConfigFile $configFile -LogDirectory $logDirectory -Round $state.validationRound
        $state.validation = @($validation.Results)
        Save-LoopState -State $state -Path $statePath
        if ($validation.Passed) {
            break
        }
        if ($NoAiRepair -or $state.repairAttempts -ge $config.maxRepairAttempts) {
            $state.phase = 'failed_validation'
            Save-LoopState -State $state -Path $statePath
            Write-UpgradeReport -State $state -Repository $worktree -Path $reportPath
            throw "Validation failed. See $reportPath"
        }

        $state.repairAttempts = [int]$state.repairAttempts + 1
        $failureContext = Get-FailureContext -LogPaths $validation.FailedLogs
        Invoke-AiRepair -Worktree $worktree -TargetVersionTag $targetTagObject.Tag `
            -Config $config -FailureContext $failureContext -LogDirectory $logDirectory `
            -Attempt $state.repairAttempts
        Save-LoopState -State $state -Path $statePath
    }

    if (-not (Test-MergeInProgress -Repository $worktree)) {
        throw 'Expected an uncommitted merge at the approval boundary, but MERGE_HEAD is absent.'
    }
    $state.phase = 'awaiting_approval'
    Save-LoopState -State $state -Path $statePath
    Write-UpgradeReport -State $state -Repository $worktree -Path $reportPath
    Write-Host "Update validated and awaiting manual approval: $reportPath"
    Write-Host "Review worktree: $worktree"
} finally {
    if ($null -ne $lockStream) {
        $lockStream.Dispose()
    }
}

[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$RepoPath,

    [Parameter(Mandatory)]
    [string]$ConfigPath,

    [Parameter(Mandatory)]
    [ValidatePattern('^v\d+\.\d+\.\d+$')]
    [string]$TargetTag
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Resolve-PathWithinRoot {
    param(
        [Parameter(Mandatory)]
        [string]$Root,

        [Parameter(Mandatory)]
        [string]$RelativePath
    )

    $rootWithSeparator = [System.IO.Path]::GetFullPath($Root).TrimEnd(
        [System.IO.Path]::DirectorySeparatorChar,
        [System.IO.Path]::AltDirectorySeparatorChar
    ) + [System.IO.Path]::DirectorySeparatorChar
    $candidate = [System.IO.Path]::GetFullPath((Join-Path $Root $RelativePath))
    if (-not $candidate.StartsWith($rootWithSeparator, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Configured path escapes the repository root: $RelativePath"
    }
    return $candidate
}

$repo = (Resolve-Path -LiteralPath $RepoPath).Path
$configFile = (Resolve-Path -LiteralPath $ConfigPath).Path
$config = Get-Content -LiteralPath $configFile -Raw -Encoding UTF8 | ConvertFrom-Json
if ($config.schemaVersion -ne 1) {
    throw "Unsupported update loop config schema: $($config.schemaVersion)"
}

$failures = [System.Collections.Generic.List[string]]::new()

foreach ($relativePath in $config.requiredFiles) {
    $fullPath = Resolve-PathWithinRoot -Root $repo -RelativePath $relativePath
    if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
        $failures.Add("Required file is missing: $relativePath")
    }
}

$strictUtf8 = [System.Text.UTF8Encoding]::new($false, $true)
foreach ($check in $config.protectedChecks) {
    $fullPath = Resolve-PathWithinRoot -Root $repo -RelativePath $check.path
    if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
        $failures.Add("Protected file is missing: $($check.path)")
        continue
    }

    try {
        $content = [System.IO.File]::ReadAllText($fullPath, $strictUtf8)
    } catch {
        $failures.Add("Protected file is not valid UTF-8: $($check.path): $($_.Exception.Message)")
        continue
    }

    foreach ($literal in $check.contains) {
        if ($content.IndexOf([string]$literal, [System.StringComparison]::Ordinal) -lt 0) {
            $failures.Add("Protected text is missing from $($check.path): $literal")
        }
    }
}

$versionPath = Resolve-PathWithinRoot -Root $repo -RelativePath 'backend/cmd/server/VERSION'
if (-not (Test-Path -LiteralPath $versionPath -PathType Leaf)) {
    $failures.Add('Required version file is missing: backend/cmd/server/VERSION')
} else {
    $actualVersion = (Get-Content -LiteralPath $versionPath -Raw -Encoding UTF8).Trim()
    $expectedVersion = $TargetTag.Substring(1)
    if ($actualVersion -ne $expectedVersion) {
        $failures.Add("backend/cmd/server/VERSION is '$actualVersion'; expected '$expectedVersion'")
    }
}

if ($failures.Count -gt 0) {
    foreach ($failure in $failures) {
        Write-Error $failure
    }
    throw "Custom boundary validation failed with $($failures.Count) error(s)."
}

Write-Output "Custom boundary validation passed for $TargetTag."

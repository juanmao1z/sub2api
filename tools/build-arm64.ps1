#Requires -Version 7.0
[CmdletBinding()]
param(
    [string]$Tag = '',
    [string]$Builder = '',
    [switch]$Print,
    [switch]$UseProxy
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$PSNativeCommandUseErrorActionPreference = $true

$repo = Split-Path -Parent $PSScriptRoot
$workspace = Split-Path -Parent $repo
$workspaceConfigPath = Join-Path $workspace 'workspace.json'
$targetPlatform = ''
if (-not (Test-Path -LiteralPath $workspaceConfigPath)) {
    throw "未找到工作区配置 '$workspaceConfigPath'。请从 D:\\Desktop\\sub2api-dev 运行，避免猜测构建目标。"
}
$workspaceConfig = Get-Content -LiteralPath $workspaceConfigPath -Raw -Encoding UTF8 | ConvertFrom-Json
$targetPlatform = [string]$workspaceConfig.target.platform
if ([string]::IsNullOrWhiteSpace($targetPlatform) -or $targetPlatform -notmatch '^linux/(amd64|arm64)$') {
    throw "workspace.json 的目标平台 '$targetPlatform' 无法用于当前 Docker 构建"
}
if (-not $Builder) { $Builder = [string]$workspaceConfig.target.buildxBuilder }
if (-not $Tag) { $Tag = [string](@($workspaceConfig.projects | Where-Object id -eq 'sub2api-custom')[0].build.defaultTag) }
if ($UseProxy) {
    $proxy = [string]$workspaceConfig.network.localProxy
    if ([string]::IsNullOrWhiteSpace($proxy)) { throw 'workspace.json 未配置 network.localProxy' }
    $noProxy = [string]$workspaceConfig.network.noProxy
    $env:HTTP_PROXY = $proxy
    $env:HTTPS_PROXY = $proxy
    $env:NO_PROXY = $noProxy
    $env:http_proxy = $proxy
    $env:https_proxy = $noProxy
    $env:no_proxy = $noProxy
}
if (-not $Builder) { $Builder = 'desktop-linux' }
if (-not $Tag) { $Tag = 'sub2api-custom:local-arm64' }

$activeContext = (& docker context show).Trim()
if ($LASTEXITCODE -ne 0 -or $activeContext -ne [string]$workspaceConfig.target.dockerContext) {
    throw "Docker context '$activeContext' 不符合 workspace.json 的 '$($workspaceConfig.target.dockerContext)'"
}
$builderOutput = (& docker buildx inspect $Builder 2>&1 | Out-String)
if ($LASTEXITCODE -ne 0) { throw "无法读取 Buildx builder '$Builder'" }
$platformLine = ($builderOutput -split "`r?`n" | Where-Object { $_ -match '^\s*Platforms:\s*' } | Select-Object -First 1)
$platforms = @()
if ($platformLine) {
    $platforms = @(($platformLine -replace '^\s*Platforms:\s*', '' -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ }))
}
if ($platforms -notcontains $targetPlatform) {
    throw "Buildx builder '$Builder' 不支持 $targetPlatform；请先启动正确的 Docker Desktop builder。"
}
Push-Location -LiteralPath $repo
try {
    $version = (Get-Content -LiteralPath 'backend/cmd/server/VERSION' -Raw -Encoding UTF8).Trim()
    $commit = (git rev-parse HEAD).Trim()
    $changes = @(git status --porcelain --untracked-files=normal)
    if ($changes.Count -gt 0) { $commit += '-dirty' }
    $date = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')

    $arguments = @(
        'buildx', 'bake', '--builder', $Builder,
        '--file', 'docker-bake.hcl',
        '--set', "app.platform=$targetPlatform",
        '--set', "app.tags=$Tag",
        '--set', "app.args.VERSION=$version",
        '--set', "app.args.COMMIT=$commit",
        '--set', "app.args.DATE=$date",
        '--set', "app.labels.org.opencontainers.image.version=$version",
        '--set', "app.labels.org.opencontainers.image.revision=$commit",
        '--set', "app.labels.org.opencontainers.image.created=$date"
    )
    if ($Print) { $arguments += '--print' }
    & docker @arguments
    if (-not $Print) {
        $imageArchitecture = (& docker image inspect $Tag --format '{{.Architecture}}/{{.Os}}').Trim()
        $platformParts = $targetPlatform -split '/', 2
        $expectedImageArchitecture = "$($platformParts[1])/$($platformParts[0])"
        if ($imageArchitecture -ne $expectedImageArchitecture) {
            throw "构建结果 '$Tag' 架构为 '$imageArchitecture'，期望 $expectedImageArchitecture"
        }
        Write-Output "Built: $Tag ($imageArchitecture) using $activeContext/$Builder target=$targetPlatform"
    }
}
finally {
    Pop-Location
}

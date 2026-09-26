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
if (Test-Path -LiteralPath $workspaceConfigPath) {
    $workspaceConfig = Get-Content -LiteralPath $workspaceConfigPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $targetPlatform = [string]$workspaceConfig.target.platform
    if ($targetPlatform -ne 'linux/arm64') { throw "workspace.json 的目标平台 '$targetPlatform' 不是 linux/arm64" }
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
        $env:https_proxy = $proxy
        $env:no_proxy = $noProxy
    }
}
if ($UseProxy -and -not (Test-Path -LiteralPath $workspaceConfigPath)) {
    throw "未找到工作区配置 '$workspaceConfigPath'，无法使用代理"
}
if (-not $Builder) { $Builder = 'desktop-linux' }
if (-not $Tag) { $Tag = 'sub2api-custom:local-arm64' }
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
}
finally {
    Pop-Location
}

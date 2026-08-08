param(
    [switch]$Apply,
    [ValidateRange(1, 1000)]
    [int]$BatchSize = 100
)

$ErrorActionPreference = 'Stop'
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptRoot
$secretFile = $env:ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_HOST_FILE

if ([string]::IsNullOrWhiteSpace($secretFile) -or -not (Test-Path -LiteralPath $secretFile -PathType Leaf)) {
    throw '请先设置 ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_HOST_FILE 指向 Docker Secret 文件。'
}

$listener = Get-NetTCPConnection -State Listen -LocalPort 18082 -ErrorAction SilentlyContinue
if ($null -ne $listener) {
    throw '检测到 18082 正在监听。请先停止应用容器后再迁移，避免新旧凭证格式并发写入。'
}

$previousSecretFile = $env:ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_FILE
$env:ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_FILE = $secretFile
try {
    Push-Location (Join-Path $projectRoot 'backend')
    $arguments = @('run', './cmd/migrate-account-credentials', '--batch-size', $BatchSize)
    if ($Apply) {
        $arguments += '--apply'
    }
    & go @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "迁移命令失败，退出码：$LASTEXITCODE"
    }
} finally {
    Pop-Location
    $env:ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_FILE = $previousSecretFile
}

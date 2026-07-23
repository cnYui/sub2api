param(
  [switch]$Foreground,
  [string]$CloudflaredPath = "C:\Program Files (x86)\cloudflared\cloudflared.exe",
  [string]$ConfigPath = "D:\CodeWorkSpace\sub2api\deploy\cloudflared-windows-aaccx.yml",
  [string]$LogDirectory = "D:\CodeWorkSpace\sub2api\deploy\logs"
)

$ErrorActionPreference = "Stop"

$tunnelId = "7f5fafd9-8a59-4013-ba42-3116dfc29463"
$credentialsPath = Join-Path $env:USERPROFILE ".cloudflared\$tunnelId.json"

if (-not (Test-Path -LiteralPath $CloudflaredPath)) {
  throw "未找到 cloudflared：$CloudflaredPath"
}

if (-not (Test-Path -LiteralPath $ConfigPath)) {
  throw "未找到 cloudflared 配置：$ConfigPath"
}

try {
  $health = Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/health -TimeoutSec 10
  if ($health.StatusCode -ne 200) {
    throw "本地 nginx health 状态不是 200：$($health.StatusCode)"
  }
} catch {
  throw "本地 127.0.0.1:8080 未就绪，先启动 nginx -> Sub2API 链路。$($_.Exception.Message)"
}

$hasToken = -not [string]::IsNullOrWhiteSpace($env:TUNNEL_TOKEN)
$hasCredentials = Test-Path -LiteralPath $credentialsPath

if (-not $hasToken -and -not $hasCredentials) {
  throw "缺少 Cloudflare Tunnel 凭证。请提供 TUNNEL_TOKEN 环境变量，或放置 credentials JSON：$credentialsPath"
}

$escapedConfigPath = $ConfigPath.Replace("\", "\\")
$existing = Get-CimInstance Win32_Process -Filter "name='cloudflared.exe'" |
  Where-Object {
    $_.CommandLine -like "*$tunnelId*" -and
    ($_.CommandLine -like "*$ConfigPath*" -or $_.CommandLine -like "*$escapedConfigPath*" -or $_.CommandLine -like "*tunnel*run*")
  } |
  Select-Object -First 1

if ($existing) {
  [pscustomobject]@{
    AlreadyRunning = $true
    ProcessId = $existing.ProcessId
    TunnelId = $tunnelId
  }
  exit 0
}

New-Item -ItemType Directory -Force -Path $LogDirectory | Out-Null
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$stdoutLog = Join-Path $LogDirectory "cloudflared-aaccx-$stamp.out.log"
$stderrLog = Join-Path $LogDirectory "cloudflared-aaccx-$stamp.err.log"

if ($hasToken) {
  $argsList = @("tunnel", "--no-autoupdate", "run")
} else {
  $argsList = @("tunnel", "--config", $ConfigPath, "--no-autoupdate", "run", $tunnelId)
}

if ($Foreground) {
  & $CloudflaredPath @argsList
  exit $LASTEXITCODE
}

$process = Start-Process `
  -FilePath $CloudflaredPath `
  -ArgumentList $argsList `
  -WindowStyle Hidden `
  -RedirectStandardOutput $stdoutLog `
  -RedirectStandardError $stderrLog `
  -PassThru

[pscustomobject]@{
  ProcessId = $process.Id
  TunnelId = $tunnelId
  UsesToken = $hasToken
  UsesCredentialsFile = (-not $hasToken)
  StdoutLog = $stdoutLog
  StderrLog = $stderrLog
}

[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [ValidateSet('Backup', 'Restore', 'Up', 'Down', 'Verify')]
  [string]$Action,
  [string]$EnvFile = ''
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($EnvFile)) {
  $EnvFile = Join-Path $PSScriptRoot '.env.openai-billing-candidate.local'
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $PSScriptRoot 'docker-compose.openai-billing-candidate.yml'
$candidateRoot = Join-Path $PSScriptRoot 'openai-billing-candidate'
$dumpDir = Join-Path $candidateRoot 'dumps'
$stateFile = Join-Path $candidateRoot 'backup-state.json'
$projectName = 'sub2api-openai-billing-candidate'
$outerSourceContainer = 'sub2api-postgres-dev'
$innerSourceContainer = 'sub2api-upstream-postgres'
$outerCandidateContainer = 'sub2api-openai-billing-outer-postgres'
$innerCandidateContainer = 'sub2api-openai-billing-inner-postgres'
$publicContainers = @('sub2api-dev', 'sub2api-upstream-latest', $outerSourceContainer, $innerSourceContainer)

function Assert-CandidatePath {
  param([Parameter(Mandatory)][string]$Path)

  $candidateFullPath = [IO.Path]::GetFullPath($candidateRoot).TrimEnd('\', '/')
  $targetFullPath = [IO.Path]::GetFullPath($Path)
  if ($targetFullPath -ne $candidateFullPath -and -not $targetFullPath.StartsWith("$candidateFullPath$([IO.Path]::DirectorySeparatorChar)", [StringComparison]::OrdinalIgnoreCase)) {
    throw "候选操作目标越界：$targetFullPath"
  }
  return $targetFullPath
}

function Ensure-CandidateDirectory {
  param([Parameter(Mandatory)][string]$Path)

  $resolved = Assert-CandidatePath -Path $Path
  New-Item -ItemType Directory -Force -Path $resolved | Out-Null
  return $resolved
}

function Invoke-Docker {
  param([Parameter(ValueFromRemainingArguments)][string[]]$Arguments)

  & docker @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "Docker 命令失败：docker $($Arguments -join ' ')"
  }
}

function Invoke-CandidateCompose {
  param([Parameter(ValueFromRemainingArguments)][string[]]$Arguments)

  if (-not (Test-Path -LiteralPath $EnvFile)) {
    throw "候选环境文件不存在：$EnvFile；请从 .env.openai-billing-candidate.local.example 创建本地文件"
  }
  & docker compose --project-name $projectName --env-file $EnvFile -f $composeFile @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "候选 Compose 命令失败：$($Arguments -join ' ')"
  }
}

function Get-ContainerStartedAt {
  param([Parameter(Mandatory)][string]$Container)

  $startedAt = (& docker inspect --format '{{.State.StartedAt}}' $Container).Trim()
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($startedAt)) {
    throw "无法读取公网容器启动时间：$Container"
  }
  return $startedAt
}

function Save-PublicRuntimeSnapshot {
  $snapshot = [ordered]@{}
  foreach ($container in $publicContainers) {
    $snapshot[$container] = Get-ContainerStartedAt -Container $container
  }

  $stateDirectory = Ensure-CandidateDirectory -Path $candidateRoot
  $snapshot | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $stateDirectory 'public-runtime-before.json') -Encoding utf8
  return $snapshot
}

function Assert-PublicRuntimeUnchanged {
  $snapshotPath = Join-Path $candidateRoot 'public-runtime-before.json'
  if (-not (Test-Path -LiteralPath $snapshotPath)) {
    throw "缺少公网运行态快照：$snapshotPath；必须先执行 Backup"
  }

  $before = Get-Content -Raw -LiteralPath $snapshotPath | ConvertFrom-Json
  foreach ($container in $publicContainers) {
    $current = Get-ContainerStartedAt -Container $container
    if ($current -ne $before.$container) {
      throw "公网容器启动时间发生变化，停止候选操作：$container"
    }
  }
}

function New-BackupDump {
  param(
    [Parameter(Mandatory)][string]$SourceContainer,
    [Parameter(Mandatory)][string]$Prefix
  )

  $safeDumpDir = Ensure-CandidateDirectory -Path $dumpDir
  $dumpName = "$Prefix-$(Get-Date -Format 'yyyyMMdd-HHmmss').dump"
  $dumpPath = Join-Path $safeDumpDir $dumpName
  $temporaryDump = "/tmp/$dumpName"

  $dumpCommand = 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges --file="{0}"' -f $temporaryDump
  Invoke-Docker exec $SourceContainer sh -lc $dumpCommand | Out-Null
  try {
    Invoke-Docker cp "${SourceContainer}:$temporaryDump" $dumpPath | Out-Null
  }
  finally {
    Invoke-Docker exec $SourceContainer sh -lc "rm -f $temporaryDump" | Out-Null
  }

  Invoke-Docker run --rm --volume "${safeDumpDir}:/dumps:ro" postgres:18-alpine pg_restore --list "/dumps/$dumpName" | Out-Null
  return $dumpName
}

function Wait-CandidatePostgres {
  param([Parameter(Mandatory)][string]$Service)

  $deadline = (Get-Date).AddSeconds(60)
  while ((Get-Date) -lt $deadline) {
    & docker compose --project-name $projectName --env-file $EnvFile -f $composeFile exec --no-TTY $Service sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "SELECT 1"' | Out-Null
    if ($LASTEXITCODE -eq 0) {
      return
    }
    Start-Sleep -Seconds 1
  }
  throw "候选 PostgreSQL 未在 60 秒内就绪：$Service"
}

function Restore-CandidateDatabase {
  param(
    [Parameter(Mandatory)][string]$Service,
    [Parameter(Mandatory)][string]$Container,
    [Parameter(Mandatory)][string]$DumpName,
    [Parameter(Mandatory)][string[]]$SQLFiles
  )

  $restoreCommand = 'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner --no-privileges "/candidate/dumps/{0}"' -f $DumpName
  Invoke-CandidateCompose exec --no-TTY $Service sh -lc $restoreCommand

  $accountsTable = (Invoke-CandidateCompose exec --no-TTY $Service sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "SELECT to_regclass(''public.accounts'')"').Trim()
  if ($accountsTable -ne 'accounts') {
    throw "候选数据库恢复后缺少 accounts 表：$Service"
  }

  foreach ($sqlFile in $SQLFiles) {
    $temporarySQL = "/tmp/$([IO.Path]::GetFileName($sqlFile))"
    Invoke-Docker cp $sqlFile "${Container}:$temporarySQL"
    try {
      $applySQLCommand = 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f "{0}"' -f $temporarySQL
      Invoke-CandidateCompose exec --no-TTY $Service sh -lc $applySQLCommand
    }
    finally {
      Invoke-Docker exec $Container sh -lc "rm -f $temporarySQL" | Out-Null
    }
  }
}

function Invoke-Backup {
  Ensure-CandidateDirectory -Path $candidateRoot | Out-Null
  $snapshot = Save-PublicRuntimeSnapshot
  $outerDump = New-BackupDump -SourceContainer $outerSourceContainer -Prefix 'outer'
  $innerDump = New-BackupDump -SourceContainer $innerSourceContainer -Prefix 'inner'
  [ordered]@{
    outer_dump = $outerDump
    inner_dump = $innerDump
    public_runtime = $snapshot
  } | ConvertTo-Json | Set-Content -LiteralPath $stateFile -Encoding utf8
  Assert-PublicRuntimeUnchanged
}

function Get-BackupState {
  if (-not (Test-Path -LiteralPath $stateFile)) {
    throw "缺少候选备份状态：$stateFile；必须先执行 Backup"
  }
  return Get-Content -Raw -LiteralPath $stateFile | ConvertFrom-Json
}

function Invoke-Restore {
  $state = Get-BackupState
  $outerDumpPath = Join-Path $dumpDir $state.outer_dump
  $innerDumpPath = Join-Path $dumpDir $state.inner_dump
  if (-not (Test-Path -LiteralPath $outerDumpPath) -or -not (Test-Path -LiteralPath $innerDumpPath)) {
    throw '候选备份文件不完整，拒绝恢复'
  }

  Invoke-CandidateCompose up --detach billing-outer-postgres billing-outer-redis billing-inner-postgres billing-inner-redis
  Wait-CandidatePostgres -Service 'billing-outer-postgres'
  Wait-CandidatePostgres -Service 'billing-inner-postgres'
  $sanitizeSQL = Join-Path $PSScriptRoot 'sql/openai-billing-candidate-sanitize.sql'
  $outerRouteSQL = Join-Path $PSScriptRoot 'sql/openai-billing-candidate-outer-route.sql'
  Restore-CandidateDatabase -Service 'billing-outer-postgres' -Container $outerCandidateContainer -DumpName $state.outer_dump -SQLFiles @($sanitizeSQL, $outerRouteSQL)
  Restore-CandidateDatabase -Service 'billing-inner-postgres' -Container $innerCandidateContainer -DumpName $state.inner_dump -SQLFiles @($sanitizeSQL)
  Assert-PublicRuntimeUnchanged
}

function Assert-Health {
  param([Parameter(Mandatory)][string]$URL)

  $response = Invoke-WebRequest -UseBasicParsing -Uri $URL -TimeoutSec 10
  if ($response.StatusCode -ne 200) {
    throw "健康检查失败：$URL -> $($response.StatusCode)"
  }
}

function Invoke-Up {
  Invoke-CandidateCompose up --detach
  Assert-Health -URL 'http://127.0.0.1:18081/health'
  Assert-Health -URL 'http://127.0.0.1:18087/health'
  Assert-PublicRuntimeUnchanged
}

function Invoke-Down {
  Invoke-CandidateCompose down --remove-orphans
  Assert-PublicRuntimeUnchanged
}

function Invoke-Verify {
  & node (Join-Path $PSScriptRoot 'verify-openai-billing-candidate.mjs')
  if ($LASTEXITCODE -ne 0) {
    throw '候选环境静态校验失败'
  }
  Invoke-CandidateCompose config | Out-Null
  Assert-PublicRuntimeUnchanged
  Assert-Health -URL 'http://127.0.0.1:18080/health'
  Assert-Health -URL 'http://127.0.0.1:18086/health'
  Assert-Health -URL 'http://127.0.0.1:18081/health'
  Assert-Health -URL 'http://127.0.0.1:18087/health'
}

switch ($Action) {
  'Backup' { Invoke-Backup }
  'Restore' { Invoke-Restore }
  'Up' { Invoke-Up }
  'Down' { Invoke-Down }
  'Verify' { Invoke-Verify }
}

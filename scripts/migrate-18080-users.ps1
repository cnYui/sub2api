param(
    [switch]$Execute,
    [string]$BackupRoot = '',
    [ValidateRange(1, 100)]
    [int]$SamplePercent = 100
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$sourcePostgres = 'sub2api-postgres-dev'
$targetPostgres = 'sub2api-official-18082-postgres'
$sourceApp = 'sub2api-dev'
$targetApp = 'sub2api-official-18082'
$sourceNetwork = 'sub2api-localdev_sub2api-network'
$sqlPath = Join-Path $PSScriptRoot 'migrate-18080-users.sql'

if (-not (Test-Path $sqlPath)) {
    throw "迁移 SQL 不存在: $sqlPath"
}

function Invoke-DockerSql {
    param(
        [string]$Container,
        [string]$Sql,
        [hashtable]$Variables = @{}
    )
    $args = @('exec', $Container, 'psql', '-U', 'sub2api', '-d', 'sub2api', '-v', 'ON_ERROR_STOP=1', '-P', 'pager=off')
    foreach ($entry in $Variables.GetEnumerator()) {
        $args += @('-v', "$($entry.Key)=$($entry.Value)")
    }
    $args += @('-c', $Sql)
    & docker @args
    if ($LASTEXITCODE -ne 0) {
        throw "数据库命令失败: $Container"
    }
}

function Test-HealthyContainer {
    param([string]$Container)
    $status = & docker inspect --format '{{.State.Health.Status}}' $Container
    if ($LASTEXITCODE -ne 0 -or $status -ne 'healthy') {
        throw "容器未处于 healthy: $Container ($status)"
    }
}

function Test-StoppedContainer {
    param([string]$Container)
    $status = & docker inspect --format '{{.State.Status}}' $Container
    if ($LASTEXITCODE -ne 0 -or $status -ne 'exited') {
        throw "正式迁移要求应用停止写入: $Container ($status)"
    }
}

Test-HealthyContainer $sourcePostgres
Test-HealthyContainer $targetPostgres

if ($Execute) {
    Test-StoppedContainer $sourceApp
    Test-StoppedContainer $targetApp
}

if ([string]::IsNullOrWhiteSpace($BackupRoot)) {
    $BackupRoot = Join-Path $repoRoot ('..\migration-backups\18080-to-18082-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
}
New-Item -ItemType Directory -Force -Path $BackupRoot | Out-Null

$sourcePassword = (& docker inspect $sourcePostgres --format '{{range .Config.Env}}{{println .}}{{end}}' | Select-String '^POSTGRES_PASSWORD=').ToString().Substring(18)
if ([string]::IsNullOrWhiteSpace($sourcePassword)) {
    throw '无法从源 PostgreSQL 容器读取连接密码'
}

Write-Host "备份源库与目标库到 $BackupRoot"
& docker exec $sourcePostgres sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc --no-owner --no-privileges > /tmp/migration-source.dump'
if ($LASTEXITCODE -ne 0) { throw '源库备份失败' }
& docker cp "$sourcePostgres`:/tmp/migration-source.dump" (Join-Path $BackupRoot 'source-18080.dump')
if ($LASTEXITCODE -ne 0) { throw '复制源库备份失败' }
& docker exec $targetPostgres sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc --no-owner --no-privileges > /tmp/migration-target.dump'
if ($LASTEXITCODE -ne 0) { throw '目标库备份失败' }
& docker cp "$targetPostgres`:/tmp/migration-target.dump" (Join-Path $BackupRoot 'target-18082-before.dump')
if ($LASTEXITCODE -ne 0) { throw '复制目标库备份失败' }

$networkAttached = $false
try {
    $networkNames = (& docker inspect $targetPostgres --format '{{json .NetworkSettings.Networks}}')
    if ($networkNames -notmatch [regex]::Escape($sourceNetwork)) {
        & docker network connect $sourceNetwork $targetPostgres
        if ($LASTEXITCODE -ne 0) { throw '将目标数据库接入源库网络失败' }
        $networkAttached = $true
    }

    $commitValue = if ($Execute) { 'true' } else { 'false' }
    if ($Execute) {
        Write-Host '执行模式：会写入 18082。源库与目标应用应在迁移窗口内停止写入。'
    } else {
        Write-Host '演练模式：事务最后会回滚，不改变 18082 数据。'
    }

    Write-Host "记录抽样比例：$SamplePercent%（用户、登录身份、API Key 全量）"
    Get-Content -Raw $sqlPath |
        & docker exec -i $targetPostgres psql -U sub2api -d sub2api -v ON_ERROR_STOP=1 -v "commit=$commitValue" -v "sample_percent=$SamplePercent" -v "source_password=$sourcePassword" -P pager=off
    if ($LASTEXITCODE -ne 0) { throw '迁移 SQL 执行失败' }
}
finally {
    if ($networkAttached) {
        & docker network disconnect $sourceNetwork $targetPostgres | Out-Null
    }
}

Write-Host "备份完成: $BackupRoot"
Write-Host '迁移脚本结束。执行模式下请继续运行应用健康检查和邮箱登录核验。'

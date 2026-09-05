#Requires -Version 5.1
<#
.SYNOPSIS
    在本机注册（或重建）每天 06:00 清理 docs/ai/context 的 Windows 计划任务。

.DESCRIPTION
    这是「本地 routine」的权威定义——清理只在本机发生，不 commit、不 push、不碰云端。
    任务每天 06:00 以隐藏窗口运行 scripts/prune-ai-context.ps1（默认保留 15 天）。

    幂等：重复运行会覆盖同名任务。换机器 / 重装系统后跑一次即可恢复。
    deploy/prune-ai-context.task.xml 是同一任务的只读导出快照（SID 已脱敏），仅作参考。

.PARAMETER At
    每天触发时刻，默认 06:00。

.PARAMETER TaskName
    任务名，默认 'Prune AI Context Docs'。

.EXAMPLE
    pwsh -File scripts/install-prune-task.ps1

.EXAMPLE
    # 卸载
    Unregister-ScheduledTask -TaskName 'Prune AI Context Docs' -Confirm:$false
#>
[CmdletBinding()]
param(
    [string]$At = '06:00',
    [string]$TaskName = 'Prune AI Context Docs'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$script = Join-Path $repoRoot 'scripts\prune-ai-context.ps1'
if (-not (Test-Path -LiteralPath $script)) {
    throw "找不到清理脚本: $script（是否在错误的仓库根运行？）"
}

# 优先用 pwsh(7+)，回退到 Windows PowerShell
$pwsh = (Get-Command pwsh -ErrorAction SilentlyContinue).Source
if (-not $pwsh) { $pwsh = Join-Path $PSHOME 'powershell.exe' }

$action = New-ScheduledTaskAction -Execute $pwsh `
    -Argument ('-NoProfile -NonInteractive -WindowStyle Hidden -File "{0}"' -f $script) `
    -WorkingDirectory $repoRoot

$trigger = New-ScheduledTaskTrigger -Daily -At $At

$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable `
    -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
    -MultipleInstances IgnoreNew -ExecutionTimeLimit (New-TimeSpan -Minutes 10)

$principal = New-ScheduledTaskPrincipal `
    -UserId ("{0}\{1}" -f $env:USERDOMAIN, $env:USERNAME) `
    -LogonType Interactive -RunLevel Limited

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
    -Settings $settings -Principal $principal -Force `
    -Description 'sub2api: 每天 06:00 清理 docs/ai/context 下超过 15 天的上下文文档（只删本地文件，不提交）。' | Out-Null

$info = Get-ScheduledTask -TaskName $TaskName | Get-ScheduledTaskInfo
Write-Host "已注册计划任务: $TaskName"
Write-Host "  脚本    : $script"
Write-Host "  每天    : $At"
Write-Host "  下次运行: $($info.NextRunTime)"
Write-Host ""
Write-Host "手动跑一次: Start-ScheduledTask -TaskName '$TaskName'"
Write-Host "预演不删  : pwsh -File scripts/prune-ai-context.ps1 -DryRun"

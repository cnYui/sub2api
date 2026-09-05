#Requires -Version 5.1
<#
.SYNOPSIS
    在本机注册（或重建）每天 06:00 瘦身 AGENTS.md 的 Windows 计划任务。

.DESCRIPTION
    这是「本地 routine」的权威定义——瘦身只在本机发生，不 commit、不 push、不碰云端。
    任务每天 06:00 以隐藏窗口运行 scripts/slim-agents-md.ps1：唤起 Claude Code（headless
    `claude -p`）按既定判据瘦身 AGENTS.md，并把被移除内容逐字归档到 docs/ai/context。
    改动只落工作区，由你审阅后手动提交。

    与 prune-ai-context 的本地任务并列，同为 06:00 触发；二者互不冲突——prune 删旧归档、
    slim 改 AGENTS.md 并新建当天归档（<15 天不会被 prune 删）。

    幂等：重复运行会覆盖同名任务。换机器 / 重装系统后跑一次即可恢复。

    **前置条件：命令行 `claude` 必须已登录。** 桌面 App 的登录与 CLI 的 OAuth 登录是两套
    独立凭据——桌面端登录不等于 CLI 登录。未登录时任务会跑到 `Not logged in` 优雅退出、
    不改任何文件。先在终端跑一次 `claude /login`（或给任务环境配 ANTHROPIC_API_KEY）。

.PARAMETER At
    每天触发时刻，默认 06:00。

.PARAMETER TaskName
    任务名，默认 'Slim AGENTS.md'。

.PARAMETER Model
    传给瘦身脚本的模型，默认 claude-opus-4-8。

.EXAMPLE
    pwsh -File scripts/install-slim-task.ps1

.EXAMPLE
    # 卸载
    Unregister-ScheduledTask -TaskName 'Slim AGENTS.md' -Confirm:$false
#>
[CmdletBinding()]
param(
    [string]$At = '06:00',
    [string]$TaskName = 'Slim AGENTS.md',
    [string]$Model = 'claude-opus-4-8'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$script = Join-Path $repoRoot 'scripts\slim-agents-md.ps1'
if (-not (Test-Path -LiteralPath $script)) {
    throw "找不到瘦身脚本: $script（是否在错误的仓库根运行？）"
}

# 优先用 pwsh(7+)，回退到 Windows PowerShell
$pwsh = (Get-Command pwsh -ErrorAction SilentlyContinue).Source
if (-not $pwsh) { $pwsh = Join-Path $PSHOME 'powershell.exe' }

$action = New-ScheduledTaskAction -Execute $pwsh `
    -Argument ('-NoProfile -NonInteractive -WindowStyle Hidden -File "{0}" -Model "{1}"' -f $script, $Model) `
    -WorkingDirectory $repoRoot

$trigger = New-ScheduledTaskTrigger -Daily -At $At

# 唤起模型比删文件慢，上限放宽到 30 分钟；笔记本在电池上也要能跑。
$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable `
    -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
    -MultipleInstances IgnoreNew -ExecutionTimeLimit (New-TimeSpan -Minutes 30)

$principal = New-ScheduledTaskPrincipal `
    -UserId ("{0}\{1}" -f $env:USERDOMAIN, $env:USERNAME) `
    -LogonType Interactive -RunLevel Limited

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
    -Settings $settings -Principal $principal -Force `
    -Description 'sub2api: 每天 06:00 唤起 Claude Code 瘦身 AGENTS.md 并归档到 docs/ai/context（只改工作区，不提交）。前置：CLI 需先 claude /login。' | Out-Null

$info = Get-ScheduledTask -TaskName $TaskName | Get-ScheduledTaskInfo
Write-Host "已注册计划任务: $TaskName"
Write-Host "  脚本    : $script"
Write-Host "  模型    : $Model"
Write-Host "  每天    : $At"
Write-Host "  下次运行: $($info.NextRunTime)"
Write-Host ""
Write-Host "前置    : 命令行 claude 需已登录（claude /login），否则任务会 Not logged in 空跑。"
Write-Host "手动跑一次: Start-ScheduledTask -TaskName '$TaskName'"
Write-Host "预演不改  : pwsh -File scripts/slim-agents-md.ps1 -DryRun"

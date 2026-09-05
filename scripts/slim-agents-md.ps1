#Requires -Version 5.1
<#
.SYNOPSIS
    对 AGENTS.md 做一次瘦身，并把瘦身记录归档到 docs/ai/context。

.DESCRIPTION
    本脚本不自己做取舍判断——它在本地无人值守地唤起 Claude Code（headless
    `claude -p`），由模型按既定判据瘦身 AGENTS.md，再把被移除内容逐字归档到
    docs/ai/context/<时间戳>-agents-md-slimming_CN.md。

    为什么要唤起模型：AGENTS.md 每个会话都会全量载入上下文，会话结束时新条目被
    追加到文件最顶部，日积月累全是一次性运维流水；判断「哪条删了会让下个会话按
    错误事实操作生产系统」需要判断力，纯脚本做不了。

    与 prune-ai-context.ps1 一致的本地约定：
      * 只改动两处——AGENTS.md 与新建的那一个归档文档；不碰代码/配置/deploy。
      * 改动只落到工作区，**不自动 commit**，由你审阅后手动提交。
      * 运行日志写到 logs/agents-md-slimming.log（*.log 已在 .gitignore）。
      * 唤起模型时只放开 Read/Edit/Write/Glob/Grep，**不放开 Bash**，所以它不能
        跑 git、不能推送、不能碰工作区里的其它文件。

    这条任务原本挂在云端 routine，但 Claude GitHub App 对本仓库只有读权限、无法
    推送/开 PR/合并（403 Resource not accessible by integration），已改为本地执行。

.PARAMETER RepoRoot
    仓库根目录。默认取脚本所在目录的上一级。

.PARAMETER Model
    唤起 Claude 用的模型。默认 claude-opus-4-8。

.PARAMETER DryRun
    预演：以只读方式唤起模型，只打印「会移除/合并什么」，不改动任何文件。

.PARAMETER NoLog
    不写运行日志。

.EXAMPLE
    pwsh -File scripts/slim-agents-md.ps1 -DryRun

.EXAMPLE
    pwsh -File scripts/slim-agents-md.ps1
#>
[CmdletBinding()]
param(
    [string]$RepoRoot,

    [string]$Model = 'claude-opus-4-8',

    [switch]$DryRun,

    [switch]$NoLog
)

$ErrorActionPreference = 'Stop'

# --- 定位仓库根目录 ---
if (-not $RepoRoot) {
    $RepoRoot = Split-Path -Parent $PSScriptRoot
}
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$agentsPath = Join-Path $RepoRoot 'AGENTS.md'
if (-not (Test-Path -LiteralPath $agentsPath)) {
    throw "找不到 AGENTS.md：$agentsPath"
}

# --- 定位 claude 可执行文件 ---
$claude = (Get-Command claude -ErrorAction SilentlyContinue).Source
if (-not $claude) {
    $fallback = Join-Path $env:USERPROFILE '.local\bin\claude.exe'
    if (Test-Path -LiteralPath $fallback) { $claude = $fallback }
}
if (-not $claude) {
    throw 'PATH 中找不到 claude，也不在 ~/.local/bin/claude.exe；无法唤起模型。'
}

# --- 时间戳（本机时区即 JST）与归档文件名 ---
$ts       = Get-Date -Format 'yyyyMMdd-HHmmss'
$tsHuman  = Get-Date -Format 'yyyy-MM-dd HH:mm'
$archiveRel = "docs/ai/context/$ts-agents-md-slimming_CN.md"

$logLines = [System.Collections.Generic.List[string]]::new()
function Note([string]$m) { Write-Host $m; $logLines.Add("[$(Get-Date -Format 'HH:mm:ss')] $m") }

Note "仓库：$RepoRoot"
Note "模型：$Model    预演：$DryRun"

# 记录运行前 AGENTS.md 是否已有未提交改动（便于事后区分是谁改的）
$dirtyBefore = $false
if (Get-Command git -ErrorAction SilentlyContinue) {
    $porcelain = & git -C $RepoRoot status --porcelain -- AGENTS.md 2>$null
    if ($porcelain) { $dirtyBefore = $true; Note '注意：运行前 AGENTS.md 已有未提交改动，本次瘦身会叠加在其上。' }
}

# --- 组织给模型的指令 ---
# 判据、绝对不能删的清单、按内容识别坑（不按编号）、无事可做就什么都不做，
# 与曾经的云端版一致；本地版去掉了 git/分支/PR/合并，只改两处文件。
$common = @"
你在本地对 `cnYui/sub2api` 仓库的 ``AGENTS.md`` 做一次瘦身。全部难度在取舍判断上：
**删错一条会让后续每个会话按错误事实去操作生产系统。** 动手前先把 ``AGENTS.md`` 完整读一遍，
再读 ``docs/ai/context/20260905-112834-agents-md-compression_CN.md``（上一次人工压缩的判据表，
第 2、3 节），沿用同一套判据，不要自创标准。

## 瘦身对象
``AGENTS.md`` 每个会话全量载入上下文；会话结束时新条目被追加到文件最顶部（``# 项目协作约定``
标题之下、``> 本文件每个会话完整载入上下文`` 引用块之上），积成一堆带日期的
``- 2026-XX-XX：……`` 条目。它们大多是一次性运维流水，且每条末尾都已指向一个独立的
``docs/ai/context/*.md``。**这些是瘦身的主要对象。**

## 移除判据（满足任一）
- 针对单个用户或单次操作的一次性执行流水（发放/刷新/取消/退款/核查/重启/部署/镜像替换），且已有对应归档
- 已被后续变更覆盖的中间状态（某个倍率的历次调整过程、反复校准过程）
- 文中已明确标注废弃/过期/已撤回、且不再有警示价值的规则
- 前端 UI 微调、页面文案调整这类不影响判断的记录

## 保留判据（满足任一）——标准就是文件自己写的那句：**下一个会话读不到这条，会做错什么？**
- 当前仍生效的协作约定 / 系统状态（部署拓扑、当前倍率值、防火墙/备份/迁移号）/ 业务规则
- **坑**：踩过一次、下次还会踩、且症状会误导排查方向
- 未完成事项

## 绝对不能删的（最容易被误判成噪音）
1. **「坑」那一章的任何一条**，尤其读起来像「已解决的历史」的那几条（如「测模型要用精确 ID，
   测错字符串比没测更误导」「schema_migrations 行数多于文件数是正常的」「核对历史扣费必须用
   usage_logs.rate_multiplier，JOIN groups.rate_multiplier 会造出假异常」「schedulerSnapshot 两个
   同名不同物」）。它们记录的是**曾经据此得出过错误结论**的陷阱，删掉就会再犯。
   **坑的编号会随内容增删漂移，一律按内容识别、不要按编号定位**；新增坑时接**当前文件里实际存在
   的最大编号**往后排，绝不重排已有编号。
2. **「负面教训（结论已撤回）」全章**——金额和方法是明确作废、不得使用的，删掉反而会被重新捡起。
3. **「待处理的计费偏差」全章**，包括标注「已过期，别照做」的段落——那句标注本身就是护栏。
4. **「计费口径」章的分组倍率表与唯一计费公式。**
5. **「未完成」章。**
6. 文件开头 ``> 本文件每个会话完整载入上下文……`` 引用块（维护规则本身）。
有疑问就保留。**宁可少删一条，不可多删一条。**

## 顶部日期条目的处理
逐条判断：纯一次性流水→整条移除并归档；含可复用的坑/规则/当前状态→把那部分事实**合并进下方对应
章节**的既有条目（措辞与该章节风格一致、不制造重复），其余流水部分移除；已在下方有等价表述→直接移除。

## 硬性约束
- 本仓库是公开仓库。归档里任何 IP/主机/桶名/Tunnel ID/密钥一律写 ``\${变量名}`` 占位；照抄原文时
  若原文已是占位符则保持不变，**绝不试图还原真实值**。
- **只允许改动两处**：``AGENTS.md`` 和你新建的那一个归档文档。不要碰任何代码/配置/deploy/其它已有文档。
- 你没有 Bash/git 工具，也不需要——不要尝试 commit、push、开分支或 PR。
"@

if ($DryRun) {
    $prompt = $common + @"

## 本次是预演
只分析、只打印：列出你会移除哪些条目（引用其开头）、哪些事实会合并进哪一章、以及你考虑过删但
决定保留的内容。**不要修改任何文件。** 若判断当前无可瘦身，直接说明原因。
"@
    $allowed = 'Read,Glob,Grep'
    $permMode = 'plan'
} else {
    $prompt = $common + @"

## 交付（本地，无 git）
1. 直接用 Edit 改 ``AGENTS.md``：移除该移除的、把可复用事实并入对应章节。
2. 用 Write 新建归档 ``$archiveRel``（``docs/ai/context/`` 只新增不覆盖，绝不改动该目录下已有文件）。
   时间戳固定用 ``$tsHuman（+09）``。内容必须含：
   1) 标题、时间、瘦身前后行数与字节数；
   2) 逐条列出**被移除的每一条**：原文**完整照抄**（不是摘要），并注明命中哪条移除判据；
   3) 被**合并进下方章节**的事实：从哪条挪到哪一章、合并后的措辞；
   4) 一节「本次刻意保留的内容」：列出你考虑过删但决定保留的条目及理由。
   目的是**归档而非删除**：任何被移除内容都必须能从这份文档完整还原。
3. **无事可做时不要硬删**：若顶部没有可移除的日期条目、正文也没有明确符合移除判据的内容，就什么都
   不做，不建归档文档，直接说明「本次无需瘦身」。AGENTS.md 已是压缩过的文件，为「有产出」而强行
   删内容比不跑还糟。
4. 最后用一段话总结：移除了几条、合并了什么、瘦身前后行数、归档文档路径（若本次无需瘦身则说明原因）。
"@
    $allowed = 'Read,Edit,Write,Glob,Grep'
    $permMode = 'acceptEdits'
}

# --- 唤起 Claude ---
Note "唤起 claude（--permission-mode $permMode --allowedTools $allowed）…"
$claudeArgs = @(
    '-p', $prompt,
    '--model', $Model,
    '--permission-mode', $permMode,
    '--allowedTools', $allowed
)

$exit = 0
try {
    Push-Location $RepoRoot
    $output = & $claude @claudeArgs 2>&1 | Out-String
    $exit = $LASTEXITCODE
} finally {
    Pop-Location
}

if ($output) {
    Write-Host $output
    foreach ($l in ($output -split "`r?`n")) { if ($l.Trim()) { $logLines.Add($l) } }
}

# --- 收尾 ---
if ($DryRun) {
    Note '预演结束：未改动任何文件。'
} elseif ($exit -ne 0) {
    Note "claude 退出码非零（$exit）——本次可能未完成瘦身，请查看上面的输出。"
} else {
    $changed = @()
    if (Get-Command git -ErrorAction SilentlyContinue) {
        $changed = & git -C $RepoRoot status --porcelain -- AGENTS.md $archiveRel 2>$null
    }
    if ($changed) {
        Note "瘦身改动已落到工作区（AGENTS.md / $archiveRel）。**未自动提交**，请审阅后手动 git add + commit。"
    } else {
        Note '模型未产生文件改动——大概率判定为「本次无需瘦身」，属正常。'
    }
}

if (-not $NoLog) {
    $logDir = Join-Path $RepoRoot 'logs'
    if (-not (Test-Path -LiteralPath $logDir)) { New-Item -ItemType Directory -Path $logDir -Force | Out-Null }
    # *.log 已在 .gitignore，日志不会进版本库
    Add-Content -LiteralPath (Join-Path $logDir 'agents-md-slimming.log') -Value $logLines -Encoding UTF8
}

exit $exit

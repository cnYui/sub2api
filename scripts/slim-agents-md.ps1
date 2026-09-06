#Requires -Version 5.1
<#
.SYNOPSIS
    对 AGENTS.md 做一次瘦身，并走完 提 PR → 合并 main → 同步本地 的闭环。

.DESCRIPTION
    完整流程（全部在本机）：
      1. 在一个基于 <Remote>/main 的**独立 git worktree**（干净检出）里唤起 Claude
         Code（headless `claude -p`）按既定判据瘦身 AGENTS.md，并把被移除内容逐字
         归档到 docs/ai/context/<时间戳>-agents-md-slimming_CN.md。
      2. 若产生改动：在 worktree 里提交这两个文件、推分支到 <Remote>、`gh pr create`
         开 PR、`gh pr merge --squash --delete-branch` 合并到 main。
      3. 移除临时 worktree，best-effort 把 <Remote>/main 快进同步回本地 main。

    为什么用独立 worktree：本仓库主工作区长期有多个会话/任务的未提交改动（另一个
    06:00 的 prune 任务会并发删旧归档，用户手上也常有未提交的代码）。在独立 worktree
    里操作，瘦身/提交/推送/合并全程**不碰主工作区**，也不会把无关改动卷进 PR。

    只让 Claude 做判断（Read/Edit/Write/Glob/Grep，**不放开 Bash**）；分支/提交/推送/
    PR/合并/同步全部由本脚本确定性执行——push 和 merge 到 main 不该有模型的不确定性。

    日志写 logs/agents-md-slimming.log（*.log 已 gitignore）。

    ─────────────────────────────────────────────────────────────────────
    前置条件（缺一不可，任务会优雅空跑并在日志说明）：
      * 命令行 `claude` 已登录：`claude /login`。**桌面 App 的登录 ≠ CLI 的登录**，
        是两套独立凭据。也可给任务环境配 ANTHROPIC_API_KEY 走 API 计费。
      * `gh` 已登录且有 repo 权限：`gh auth login`。
      * 本地能推 <Remote>（git 凭据有效）。
    ─────────────────────────────────────────────────────────────────────

.PARAMETER RepoRoot
    仓库根目录。默认取脚本所在目录的上一级。

.PARAMETER Model
    唤起 Claude 用的模型。默认 claude-opus-4-8。

.PARAMETER Remote
    推送/开 PR 的 git 远端名。默认 fork（本仓库指向 cnYui/sub2api 的远端）。

.PARAMETER RepoSlug
    GitHub owner/repo，供 gh 显式定位（本仓库有多个远端，不能靠自动解析）。默认 cnYui/sub2api。

.PARAMETER DryRun
    预演：在 worktree 里真的跑瘦身并打印 diff，但**不提交、不推送、不开 PR、不合并**，最后清理 worktree。

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
    [string]$Model    = 'claude-opus-4-8',
    [string]$Remote   = 'fork',
    [string]$RepoSlug = 'cnYui/sub2api',
    [switch]$DryRun,
    [switch]$NoLog
)

$ErrorActionPreference = 'Stop'
# 原生命令（git/gh/claude）会往 stderr 打正常进度（如 "Preparing worktree…"）。
# pwsh7 在 EAP=Stop 下会把这些 stderr 当致命错误抛出，这里关掉该行为，改为只依据
# $LASTEXITCODE 判断成败（每个调用点都显式检查）。
if (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue) {
    $PSNativeCommandUseErrorActionPreference = $false
}

# --- 定位仓库根目录、claude、git、gh ---
if (-not $RepoRoot) { $RepoRoot = Split-Path -Parent $PSScriptRoot }
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
if (-not (Test-Path -LiteralPath (Join-Path $RepoRoot 'AGENTS.md'))) {
    throw "找不到 AGENTS.md：$RepoRoot\AGENTS.md"
}
$claude = (Get-Command claude -ErrorAction SilentlyContinue).Source
if (-not $claude) {
    $fallback = Join-Path $env:USERPROFILE '.local\bin\claude.exe'
    if (Test-Path -LiteralPath $fallback) { $claude = $fallback }
}
if (-not $claude) { throw '找不到 claude 可执行文件。' }
$gitExe = @(Get-Command git -CommandType Application -ErrorAction SilentlyContinue)[0].Source
if (-not $gitExe) { throw '找不到 git。' }
$ghExe = @(Get-Command gh -CommandType Application -ErrorAction SilentlyContinue)[0].Source
$ghAvailable = [bool]$ghExe

$ts        = Get-Date -Format 'yyyyMMdd-HHmmss'
$tsHuman   = Get-Date -Format 'yyyy-MM-dd HH:mm'
$dateOnly  = Get-Date -Format 'yyyyMMdd'
$branch    = "docs/agents-md-slimming-$dateOnly"
$archiveRel = "docs/ai/context/$ts-agents-md-slimming_CN.md"
$worktree  = Join-Path $env:TEMP "sub2api-slim-$ts"

$logLines = [System.Collections.Generic.List[string]]::new()
function Note([string]$m) { Write-Host $m; $logLines.Add("[$(Get-Date -Format 'HH:mm:ss')] $m") }

# 两个坑一起处理：
# 1) PowerShell 函数名大小写不敏感，函数体内绝不能写裸 `git`/`gh`（会解析回本函数导致
#    无限递归 / call depth overflow），必须用解析出的 $gitExe/$ghExe 路径调用。
# 2) git/gh 会往 stderr 打正常进度（"Preparing worktree…" 等）；在 EAP=Stop 下 pwsh 会把
#    这些 stderr 当致命错误抛出。所以每次原生调用都在局部 EAP=Continue 下执行，成败只看
#    $LASTEXITCODE（每个调用点都显式检查）。
# 全部用「简单函数 + $args」，不写 param()/[Parameter]——否则会变成高级函数、带上
# -PipelineVariable 等公共参数，传入的 `-p`/`-C` 会被当参数前缀去绑定而报错。
function Invoke-Native {
    $exe  = $args[0]
    $rest = @($args | Select-Object -Skip 1)
    $old = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
    try { & $exe @rest 2>&1 } finally { $ErrorActionPreference = $old }
}
function Git   { Invoke-Native $gitExe @args }
function GitWt { Invoke-Native $gitExe -C $worktree @args }
function Gh    { Invoke-Native $ghExe @args }

function Write-Log {
    if ($NoLog) { return }
    $logDir = Join-Path $RepoRoot 'logs'
    if (-not (Test-Path -LiteralPath $logDir)) { New-Item -ItemType Directory -Path $logDir -Force | Out-Null }
    Add-Content -LiteralPath (Join-Path $logDir 'agents-md-slimming.log') -Value $logLines -Encoding UTF8
}

Note "仓库：$RepoRoot    远端：$Remote($RepoSlug)    模型：$Model    预演：$DryRun"

# --- 唤起 Claude 用的瘦身指令（判断部分，与既定判据一致；不含任何 git 操作）---
$prompt = @"
你在一个基于 $Remote/main 的**干净 worktree**（当前目录）里，对 ``AGENTS.md`` 做一次瘦身。
全部难度在取舍判断上：**删错一条会让后续每个会话按错误事实去操作生产系统。**
动手前先把 ``AGENTS.md`` 完整读一遍，再读 ``docs/ai/context/20260905-112834-agents-md-compression_CN.md``
（上一次人工压缩的判据表，第 2、3 节），沿用同一套判据，不要自创标准。

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
1. **「坑」那一章的任何一条**，尤其读起来像「已解决的历史」的那几条（如「测模型要用精确 ID，测错
   字符串比没测更误导」「schema_migrations 行数多于文件数是正常的」「核对历史扣费必须用
   usage_logs.rate_multiplier，JOIN groups.rate_multiplier 会造出假异常」「schedulerSnapshot 两个同名
   不同物」）。它们记录的是**曾经据此得出过错误结论**的陷阱，删掉就会再犯。
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
- 你没有 Bash/git 工具，也不需要——不要尝试 commit/push/开分支或 PR，那些由外层脚本处理。

## 交付
1. 用 Edit 改 ``AGENTS.md``：移除该移除的、把可复用事实并入对应章节。
2. 用 Write 新建归档 ``$archiveRel``（``docs/ai/context/`` 只新增不覆盖）。时间戳固定用
   ``$tsHuman（+09）``。内容必须含：① 标题、时间、瘦身前后行数与字节数；② 逐条列出被移除的每一条，
   原文**完整照抄**（不是摘要）并注明命中哪条移除判据；③ 被合并进下方章节的事实（从哪条挪到哪一章、
   合并后措辞）；④ 一节「本次刻意保留的内容」。目的是**归档而非删除**，任何被移除内容都要能从这份
   文档完整还原。
3. **无事可做时不要硬删**：若顶部没有可移除的日期条目、正文也没有明确符合移除判据的内容，就什么都
   不做、不建归档文档。AGENTS.md 已是压缩过的文件，为「有产出」而强行删内容比不跑还糟。
"@

$prCreated = $null
$didSlim   = $false
$merged    = $false

try {
    Note "git fetch $Remote …"
    $f = Git fetch $Remote --quiet
    if ($LASTEXITCODE -ne 0) { Note "fetch 失败：$f"; throw "无法 fetch $Remote" }

    # 清理可能残留的同名分支 / worktree
    Git worktree prune | Out-Null
    if (Git rev-parse --verify --quiet "refs/heads/$branch") { Git branch -D $branch | Out-Null }

    Note "创建 worktree（$Remote/main）：$worktree  分支 $branch"
    $w = Git worktree add $worktree -b $branch "$Remote/main"
    if ($LASTEXITCODE -ne 0) { Note "worktree add 失败：$w"; throw 'worktree 创建失败' }

    try {
        # --- 唤起 Claude 在 worktree 里瘦身 ---
        Note '唤起 claude（Read/Edit/Write/Glob/Grep，acceptEdits，不含 Bash）…'
        Push-Location $worktree
        try {
            $out = (Invoke-Native $claude -p $prompt --model $Model --permission-mode acceptEdits --allowedTools 'Read,Edit,Write,Glob,Grep') | Out-String
            $claudeExit = $LASTEXITCODE
        } finally { Pop-Location }
        if ($out) { Write-Host $out; foreach ($l in ($out -split "`r?`n")) { if ($l.Trim()) { $logLines.Add($l) } } }
        if ($claudeExit -ne 0) { Note "claude 退出码非零（$claudeExit）——未登录或其它错误，终止本次（不提交/不 PR）。"; throw 'claude 失败' }

        $changes = GitWt status --porcelain
        if (-not $changes) { Note '模型未产生改动——判定「本次无需瘦身」，正常结束。'; return }
        $didSlim = $true
        Note "worktree 内改动：`n$($changes | Out-String)"

        if ($DryRun) {
            Note '预演 diff（AGENTS.md）：'
            Write-Host ((GitWt --no-pager diff -- AGENTS.md) | Out-String)
            Note '预演：不提交、不推送、不开 PR。'
            return
        }

        # --- 提交（只加这两个路径）、推送、开 PR、合并 ---
        $before = (GitWt show "$Remote/main:AGENTS.md" | Measure-Object -Line).Lines
        $after  = (Get-Content -LiteralPath (Join-Path $worktree 'AGENTS.md') | Measure-Object -Line).Lines
        GitWt add -- AGENTS.md $archiveRel | Out-Null
        $commitMsg = @"
docs: 瘦身 AGENTS.md（$before → $after 行）

自动瘦身：移除顶部一次性运维流水条目，可复用事实并入正文对应章节，
被移除原文逐字归档到 $archiveRel。由 scripts/slim-agents-md.ps1 生成。

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
"@
        $c = GitWt commit -m $commitMsg
        if ($LASTEXITCODE -ne 0) { Note "commit 失败：$c"; throw 'commit 失败' }

        Note "推送分支 $branch → $Remote …"
        $p = GitWt push -u $Remote $branch
        if ($LASTEXITCODE -ne 0) { Note "push 失败：$p"; throw 'push 失败' }

        if (-not $ghAvailable) { Note '找不到 gh，分支已推送但无法自动开 PR/合并，请手动处理。'; throw 'gh 缺失' }

        $prBody = @"
自动瘦身 AGENTS.md：移除顶部一次性运维流水条目，可复用事实并入正文对应章节。
被移除条目的原文逐字归档在 ``$archiveRel``（只新增不覆盖）。
AGENTS.md：$before → $after 行。由 scripts/slim-agents-md.ps1 于 $tsHuman（+09）生成。

🤖 Generated with [Claude Code](https://claude.com/claude-code)
"@
        Note '开 PR …'
        $prCreated = (Gh pr create --repo $RepoSlug --base main --head $branch --title "docs: AGENTS.md 每日瘦身 $dateOnly" --body $prBody) | Out-String
        if ($LASTEXITCODE -ne 0) { Note "gh pr create 失败：$prCreated"; throw 'PR 创建失败' }
        $prCreated = $prCreated.Trim()
        Note "PR：$prCreated"

        Note '合并 PR（squash，删分支）…'
        $m = (Gh pr merge $branch --repo $RepoSlug --squash --delete-branch) | Out-String
        if ($LASTEXITCODE -ne 0) { Note "gh pr merge 失败：$m（PR 已开，留待人工合并）"; throw '合并失败' }
        $merged = $true
        Note "已合并到 main：$prCreated"
    }
    finally {
        if (Test-Path -LiteralPath $worktree) {
            Git worktree remove $worktree --force | Out-Null
            Note "已移除 worktree：$worktree"
        }
        Git worktree prune | Out-Null
        if (Git rev-parse --verify --quiet "refs/heads/$branch") { Git branch -D $branch | Out-Null }
    }

    # --- best-effort 同步远端 main 回本地 main ---
    if ($merged) {
        Git fetch $Remote --quiet | Out-Null
        $curBranch = (Git rev-parse --abbrev-ref HEAD).Trim()
        if ($curBranch -ne 'main') {
            Note "本地当前不在 main（在 $curBranch），跳过自动同步。远端已合并，需要时手动 git pull。"
        } else {
            $mrg = Git merge --ff-only "$Remote/main"
            if ($LASTEXITCODE -eq 0) { Note "本地 main 已快进同步到 $Remote/main。" }
            else { Note "本地 main 无法快进同步（工作区可能有未提交改动，或已分叉）：$mrg`n远端已合并，请手动 git pull。" }
        }
    }
}
catch {
    Note "本次未走完流程：$($_.Exception.Message)"
}
finally {
    Write-Log
}

if ($merged) { exit 0 } elseif ($didSlim) { exit 1 } else { exit 0 }

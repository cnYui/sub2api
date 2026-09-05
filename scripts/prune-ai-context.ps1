#Requires -Version 5.1
<#
.SYNOPSIS
    清理 docs/ai/context 下超过保留期的历史上下文文档。

.DESCRIPTION
    覆盖该目录下的全部文件（.md 正文和随文导出的 .csv 等）。文档年龄以文件名前缀
    YYYYMMDD 为准（这是本仓库的命名约定），缺前缀时回退到文件最后修改时间。
    默认保留 15 天，更早的一律删除。

    两类文档会被跳过，不会被删：
      1) 仍被 AGENTS.md / CLAUDE.md / README.md 或 docs 下（context 目录之外）
         任意 Markdown 引用的文档——AGENTS.md 里写着「动生产数据库前先读它」这类
         指路条目，删掉引用目标会让规则文件指向不存在的文件。
      2) 正文里带 <!-- prune:keep --> 标记的文档。

    已被 git 跟踪的文档删掉后还能从历史里找回；未跟踪的删掉就没了，所以
    -SkipUntracked 可以只保留未跟踪的那部分。

.PARAMETER Days
    保留天数，默认 15。严格超过该天数才删除（第 15 天当天的文档保留）。

.PARAMETER DryRun
    只打印将要删除的清单，不实际删除。

.PARAMETER SkipUntracked
    跳过未被 git 跟踪的文档（这些文档删除后无法从 git 历史恢复）。

.EXAMPLE
    pwsh -File scripts/prune-ai-context.ps1 -DryRun

.EXAMPLE
    pwsh -File scripts/prune-ai-context.ps1
#>
[CmdletBinding()]
param(
    [ValidateRange(1, 3650)]
    [int]$Days = 15,

    [string]$RepoRoot,

    [string]$RelativePath = 'docs/ai/context',

    [switch]$DryRun,

    [switch]$SkipUntracked,

    [switch]$NoLog
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Resolve-RepoRoot {
    param([string]$Explicit, [string]$ScriptDir)

    if ($Explicit) {
        if (-not (Test-Path -LiteralPath $Explicit)) {
            throw "指定的 RepoRoot 不存在: $Explicit"
        }
        return (Resolve-Path -LiteralPath $Explicit).Path
    }

    # 优先问 git，脚本被软链或从别处调用时也能定位到正确的仓库根。
    try {
        $top = & git -C $ScriptDir rev-parse --show-toplevel 2>$null
        if ($LASTEXITCODE -eq 0 -and $top) {
            return (Resolve-Path -LiteralPath ($top | Select-Object -First 1)).Path
        }
    } catch {
        # git 不可用时退回目录推断
    }

    return (Resolve-Path -LiteralPath (Join-Path $ScriptDir '..')).Path
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-RepoRoot -Explicit $RepoRoot -ScriptDir $scriptDir
$contextDir = Join-Path $repoRoot ($RelativePath -replace '/', [IO.Path]::DirectorySeparatorChar)

if (-not (Test-Path -LiteralPath $contextDir -PathType Container)) {
    Write-Host "目录不存在，跳过: $contextDir"
    exit 0
}

$cutoff = (Get-Date).Date.AddDays(-$Days)

# ---------------------------------------------------------------------------
# 收集仍被引用的文档名
# ---------------------------------------------------------------------------
$referenceRegex = '[0-9]{8}(?:-[0-9]{6})?-[^\s"''()\[\]<>|,;]+\.[A-Za-z0-9]{1,6}'
$referenced = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase)

$guardFiles = New-Object System.Collections.Generic.List[string]
Get-ChildItem -LiteralPath $repoRoot -Filter '*.md' -File -ErrorAction SilentlyContinue |
    ForEach-Object { $guardFiles.Add($_.FullName) }

$docsDir = Join-Path $repoRoot 'docs'
if (Test-Path -LiteralPath $docsDir -PathType Container) {
    Get-ChildItem -LiteralPath $docsDir -Filter '*.md' -File -Recurse -ErrorAction SilentlyContinue |
        Where-Object { -not $_.FullName.StartsWith($contextDir, [StringComparison]::OrdinalIgnoreCase) } |
        ForEach-Object { $guardFiles.Add($_.FullName) }
}

foreach ($guard in $guardFiles) {
    $text = Get-Content -LiteralPath $guard -Raw -Encoding UTF8 -ErrorAction SilentlyContinue
    if (-not $text) { continue }
    foreach ($match in [regex]::Matches($text, $referenceRegex)) {
        [void]$referenced.Add((Split-Path $match.Value -Leaf))
    }
}

# ---------------------------------------------------------------------------
# 收集未被 git 跟踪的文档名
# ---------------------------------------------------------------------------
$untracked = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::OrdinalIgnoreCase)
$gitAvailable = $false
try {
    $lines = & git -C $repoRoot ls-files --others --exclude-standard -- $RelativePath 2>$null
    if ($LASTEXITCODE -eq 0) {
        $gitAvailable = $true
        foreach ($line in $lines) {
            if ($line) { [void]$untracked.Add((Split-Path $line -Leaf)) }
        }
    }
} catch {
    # 非 git 环境下把所有文件当作已跟踪处理，只影响 -SkipUntracked 的判断
}

# ---------------------------------------------------------------------------
# 分类
# ---------------------------------------------------------------------------
$toDelete = New-Object System.Collections.Generic.List[object]
$keptReferenced = New-Object System.Collections.Generic.List[string]
$keptMarked = New-Object System.Collections.Generic.List[string]
$keptUntracked = New-Object System.Collections.Generic.List[string]
$totalCount = 0

foreach ($file in Get-ChildItem -LiteralPath $contextDir -File) {
    $totalCount++

    if ($file.Name -match '^(\d{4})(\d{2})(\d{2})') {
        try {
            $docDate = [datetime]::new([int]$Matches[1], [int]$Matches[2], [int]$Matches[3])
        } catch {
            $docDate = $file.LastWriteTime.Date
        }
    } else {
        $docDate = $file.LastWriteTime.Date
    }

    if ($docDate -ge $cutoff) { continue }

    if ($referenced.Contains($file.Name)) {
        $keptReferenced.Add($file.Name)
        continue
    }

    $content = Get-Content -LiteralPath $file.FullName -Raw -Encoding UTF8 -ErrorAction SilentlyContinue
    if ($content -and $content -match '<!--\s*prune:keep\s*-->') {
        $keptMarked.Add($file.Name)
        continue
    }

    if ($SkipUntracked -and $untracked.Contains($file.Name)) {
        $keptUntracked.Add($file.Name)
        continue
    }

    $toDelete.Add([pscustomobject]@{
        Name      = $file.Name
        FullName  = $file.FullName
        DocDate   = $docDate
        Untracked = $untracked.Contains($file.Name)
    })
}

# ---------------------------------------------------------------------------
# 执行
# ---------------------------------------------------------------------------
$mode = if ($DryRun) { 'DRY-RUN' } else { 'DELETE' }
$stamp = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
$header = "[$stamp] $mode 保留 $Days 天（截止 $($cutoff.ToString('yyyy-MM-dd'))）；扫描 $totalCount，待删 $($toDelete.Count)"

$logLines = New-Object System.Collections.Generic.List[string]
$logLines.Add($header)

Write-Host $header

$deleted = 0
$failed = New-Object System.Collections.Generic.List[string]
foreach ($item in $toDelete) {
    $tag = if ($item.Untracked) { ' [untracked]' } else { '' }
    Write-Host "  - $($item.Name)$tag"
    $logLines.Add("  - $($item.Name)$tag")

    if (-not $DryRun) {
        try {
            Remove-Item -LiteralPath $item.FullName -Force
            $deleted++
        } catch {
            $failed.Add("$($item.Name): $($_.Exception.Message)")
        }
    }
}

foreach ($pair in @(
    @{ List = $keptReferenced; Label = '仍被 AGENTS.md/docs 引用，保留' },
    @{ List = $keptMarked;     Label = '带 prune:keep 标记，保留' },
    @{ List = $keptUntracked;  Label = '未被 git 跟踪，按 -SkipUntracked 保留' }
)) {
    if ($pair.List.Count -gt 0) {
        $line = "  $($pair.Label): $($pair.List.Count) 个 -> $($pair.List -join ', ')"
        Write-Host $line
        $logLines.Add($line)
    }
}

if ($failed.Count -gt 0) {
    foreach ($f in $failed) {
        $line = "  删除失败: $f"
        Write-Warning $line
        $logLines.Add($line)
    }
}

$summary = if ($DryRun) {
    "预演结束：将删除 $($toDelete.Count) 个文件（未实际删除）"
} else {
    "已删除 $deleted 个文件，失败 $($failed.Count) 个，剩余 $($totalCount - $deleted) 个"
}
Write-Host $summary
$logLines.Add($summary)

if ($gitAvailable -and -not $DryRun -and $deleted -gt 0) {
    Write-Host "提示：删除已落到工作区，需要 git commit 才会进入仓库历史。"
}

if (-not $NoLog) {
    $logDir = Join-Path $repoRoot 'logs'
    if (-not (Test-Path -LiteralPath $logDir)) {
        New-Item -ItemType Directory -Path $logDir -Force | Out-Null
    }
    # *.log 已在 .gitignore 中，日志不会进版本库
    Add-Content -LiteralPath (Join-Path $logDir 'prune-ai-context.log') -Value $logLines -Encoding UTF8
}

exit 0

#!/usr/bin/env bash
# 清理 docs/ai/context 下超过保留期的历史上下文文档（prune-ai-context.ps1 的 POSIX 版）。
#
# 文档年龄以文件名前缀 YYYYMMDD 为准，缺前缀时回退到文件 mtime。默认保留 15 天。
# 跳过两类文件：仍被 AGENTS.md / CLAUDE.md / docs 下（context 之外）Markdown 引用的，
# 以及正文带 <!-- prune:keep --> 标记的。
#
# 用法：
#   scripts/prune-ai-context.sh --dry-run
#   scripts/prune-ai-context.sh --days 15
set -euo pipefail

DAYS=15
DRY_RUN=0
SKIP_UNTRACKED=0
NO_LOG=0
REL_PATH="docs/ai/context"
REPO_ROOT=""

while [ $# -gt 0 ]; do
  case "$1" in
    --days)            DAYS="$2"; shift 2 ;;
    --dry-run)         DRY_RUN=1; shift ;;
    --skip-untracked)  SKIP_UNTRACKED=1; shift ;;
    --no-log)          NO_LOG=1; shift ;;
    --path)            REL_PATH="$2"; shift 2 ;;
    --repo-root)       REPO_ROOT="$2"; shift 2 ;;
    -h|--help)         sed -n '2,14p' "$0"; exit 0 ;;
    *) echo "未知参数: $1" >&2; exit 2 ;;
  esac
done

case "$DAYS" in
  ''|*[!0-9]*) echo "--days 必须是正整数" >&2; exit 2 ;;
esac
[ "$DAYS" -ge 1 ] || { echo "--days 必须 >= 1" >&2; exit 2; }

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -z "$REPO_ROOT" ]; then
  REPO_ROOT="$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null || dirname "$script_dir")"
fi

context_dir="$REPO_ROOT/$REL_PATH"
if [ ! -d "$context_dir" ]; then
  echo "目录不存在，跳过: $context_dir"
  exit 0
fi

# 截止日期：严格早于它的才删（第 DAYS 天当天保留）
if date -d 'now' +%Y%m%d >/dev/null 2>&1; then
  cutoff="$(date -d "-${DAYS} days" +%Y%m%d)"          # GNU date
else
  cutoff="$(date -v-"${DAYS}"d +%Y%m%d)"               # BSD/macOS date
fi

# --- 收集仍被引用的文件名（一次性读进关联数组，避免每个文件都 fork 一次 grep）---
declare -A REFERENCED=()
declare -A UNTRACKED=()

while IFS= read -r name; do
  if [ -n "$name" ]; then REFERENCED["$name"]=1; fi
done < <(
  {
    find "$REPO_ROOT" -maxdepth 1 -name '*.md' -type f -print0
    if [ -d "$REPO_ROOT/docs" ]; then
      find "$REPO_ROOT/docs" -name '*.md' -type f -not -path "$context_dir/*" -print0
    fi
  } | xargs -0 -r grep -ohE '[0-9]{8}(-[0-9]{6})?-[^[:space:]"'"'"'()<>|,;]+\.[A-Za-z0-9]{1,6}' 2>/dev/null     | sed 's|.*/||' | sort -u
)

while IFS= read -r name; do
  if [ -n "$name" ]; then UNTRACKED["$name"]=1; fi
done < <(
  git -C "$REPO_ROOT" ls-files --others --exclude-standard -- "$REL_PATH" 2>/dev/null | sed 's|.*/||'
)

declare -A MARKED=()
while IFS= read -r path; do
  if [ -n "$path" ]; then MARKED["${path##*/}"]=1; fi
done < <(
  grep -rlE '<!--[[:space:]]*prune:keep[[:space:]]*-->' "$context_dir" 2>/dev/null || true
)

# --- 分类并执行 -----------------------------------------------------------
mode="DELETE"
if [ "$DRY_RUN" -eq 1 ]; then mode="DRY-RUN"; fi
total=0; deleted=0; kept_ref=0; kept_mark=0; kept_untracked=0
body=""

while IFS= read -r path; do
  name="${path##*/}"
  total=$((total + 1))

  if [[ "$name" =~ ^[0-9]{8} ]]; then
    doc_date="${name:0:8}"
  else
    doc_date="$(date -r "$path" +%Y%m%d 2>/dev/null || echo "$cutoff")"
  fi
  [ "$doc_date" \< "$cutoff" ] || continue

  if [ -n "${REFERENCED[$name]:-}" ]; then
    kept_ref=$((kept_ref + 1)); continue
  fi
  if [ -n "${MARKED[$name]:-}" ]; then
    kept_mark=$((kept_mark + 1)); continue
  fi
  is_untracked=0
  if [ -n "${UNTRACKED[$name]:-}" ]; then is_untracked=1; fi
  if [ "$SKIP_UNTRACKED" -eq 1 ] && [ "$is_untracked" -eq 1 ]; then
    kept_untracked=$((kept_untracked + 1)); continue
  fi

  tag=""
  if [ "$is_untracked" -eq 1 ]; then tag=" [untracked]"; fi
  body="${body}  - ${name}${tag}"$'\n'
  if [ "$DRY_RUN" -eq 0 ]; then
    rm -f -- "$path"
    deleted=$((deleted + 1))
  fi
done < <(find "$context_dir" -maxdepth 1 -type f | sort)

pending="$(printf '%s' "$body" | grep -c . || true)"
header="[$(date '+%Y-%m-%d %H:%M:%S')] $mode 保留 ${DAYS} 天（截止 ${cutoff}）；扫描 ${total}，待删 ${pending}"

out="${header}"$'\n'"${body}"
[ "$kept_ref" -gt 0 ]       && out="${out}  仍被 AGENTS.md/docs 引用，保留: ${kept_ref} 个"$'\n'
[ "$kept_mark" -gt 0 ]      && out="${out}  带 prune:keep 标记，保留: ${kept_mark} 个"$'\n'
[ "$kept_untracked" -gt 0 ] && out="${out}  未被 git 跟踪，按 --skip-untracked 保留: ${kept_untracked} 个"$'\n'
if [ "$DRY_RUN" -eq 1 ]; then
  out="${out}预演结束：将删除 ${pending} 个文件（未实际删除）"
else
  out="${out}已删除 ${deleted} 个文件，剩余 $((total - deleted)) 个"
fi

printf '%s\n' "$out"

if [ "$NO_LOG" -eq 0 ]; then
  mkdir -p "$REPO_ROOT/logs"   # *.log 已在 .gitignore 中
  printf '%s\n' "$out" >> "$REPO_ROOT/logs/prune-ai-context.log"
fi

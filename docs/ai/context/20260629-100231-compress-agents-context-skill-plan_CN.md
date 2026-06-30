# Compress Agents Context Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use writing-skills when modifying this skill. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 创建可复用的 `compress-agents-context` skill，用于把项目 `AGENTS.md` 压缩保存到 `docs/ai/context/`。

**Architecture:** `SKILL.md` 承载语义流程和压缩标准，`scripts/validate_agents_context.py` 提供确定性验证。skill 安装在 `/Users/wujianxiang/.codex/skills/compress-agents-context`，可由 Codex 自动发现。

**Tech Stack:** Codex skill Markdown、Python 3 标准库、项目内 `docs/ai/context/` 约定。

---

## 文件结构

- Create: `/Users/wujianxiang/.codex/skills/compress-agents-context/SKILL.md`
- Create: `/Users/wujianxiang/.codex/skills/compress-agents-context/scripts/validate_agents_context.py`
- Create: `/Users/wujianxiang/.codex/skills/compress-agents-context/agents/openai.yaml`
- Create: `docs/ai/context/20260629-100230-compress-agents-context-skill-design_CN.md`
- Create: `docs/ai/context/20260629-100231-compress-agents-context-skill-plan_CN.md`

## 任务

- [ ] 初始化 skill 目录，包含 `scripts/` 资源目录和 `agents/openai.yaml`。
- [ ] 写入 `SKILL.md`，覆盖触发条件、固定流程、压缩标准、敏感信息规则、回报格式。
- [ ] 写入验证脚本，检查输出文档是否位于 `docs/ai/context/`、命名是否符合约定、是否匹配常见敏感模式。
- [ ] 运行官方 `quick_validate.py` 校验 skill 元数据。
- [ ] 运行验证脚本检查刚生成的 AGENTS 精简文档。
- [ ] 运行 `git diff --check` 和 `git status --short`，汇报未跟踪文件。

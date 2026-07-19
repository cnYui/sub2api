# Sub2API CLIProxyAPI 部署 Runbook 文档 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增一份新人可按步骤执行的 Sub2API + CLIProxyAPI 部署 Runbook，并把历史 `auth_unavailable` / 502 根因写入长期上下文。

**Architecture:** 只做文档改动，不部署、不改运行态。正式 Runbook 放在 `docs/`，入口索引放在 `deploy/README.md` 和 `AGENTS.md`；本轮计划和结果继续沉淀到 `docs/ai/context/`。

**Tech Stack:** Markdown、Docker Compose、Sub2API、CLIProxyAPI、PostgreSQL、Redis、Nginx、Cloudflare Tunnel。

---

### Task 1: 梳理现有部署事实

**Files:**
- Read: `deploy/README.md`
- Read: `deploy/DOCKER.md`
- Read: `deploy/docker-compose.candidate.yml`
- Read: `deploy/rehearse-sub2api-candidate.sh`
- Read: `deploy/promote-sub2api-candidate.sh`
- Read: `docs/ai/context/20260717-093308-agents-memory-condensed_CN.md`
- Read: `/Users/wujianxiang/.codex/attachments/991f62d3-2ad7-409b-b9ff-bd85d0780442/pasted-text.txt`

- [x] **Step 1: 读取附件事故记录**

  结论：CLIProxyAPI 本地 HTTPS 8317 与容器内直连均可用；历史公网 502 不是 CLIProxyAPI 调度器坏，而是 Sub2API account 1 被失败状态/调度快照排除，日志出现 `excluded_account_count=1`。

- [x] **Step 2: 读取当前部署脚本**

  结论：`promote-sub2api-candidate.sh` 默认目标是 `public_candidate_18084`，只替换 `sub2api-candidate` 应用容器；`rehearse-sub2api-candidate.sh` 是候选预演脚本，运行前必须核对它的生产 dump 来源。

- [x] **Step 3: 核对当前链路**

  当前链路：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。容器内访问 CLIProxyAPI 使用 `https://host.docker.internal:8317/v1`。

### Task 2: 新增正式部署 Runbook

**Files:**
- Create: `docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md`

- [x] **Step 1: 写明职责边界**

  必须写清 Sub2API 是公网入口、用户 Key、计费和用量事实源；CLIProxyAPI 是内网账号池、OAuth、协议转换、账号调度上游。

- [x] **Step 2: 写部署前红线与备份**

  必须覆盖 git 状态、容器/project/volume/端口确认、Nginx 指向、Postgres/Redis 备份和备份可读验证。

- [x] **Step 3: 写配置要求**

  必须覆盖 `https://host.docker.internal:8317/v1`、8317 当前是 TLS、Sub2API 用户 Key 不能直接打 CLIProxyAPI、Sub2API upstream key 必须与 CLIProxyAPI 内部 key 对齐。

- [x] **Step 4: 写部署和验收流程**

  必须覆盖构建镜像、dry-run 发布、只替换 18084 应用容器、health、真实公网 `/v1/chat/completions`、`usage_facts` settled 验证。

- [x] **Step 5: 写 `auth_unavailable` / 502 排障章节**

  必须覆盖历史根因：account 1 被 Sub2API 临时失败状态/Redis 调度快照排除，`excluded_account_count=1`；恢复时先备份和确认，只清临时失败/调度状态，不删业务数据。

### Task 3: 增加入口索引和长期记忆

**Files:**
- Modify: `deploy/README.md`
- Modify: `AGENTS.md`

- [x] **Step 1: 在部署 README 增加 Runbook 链接**

  入口文字应指向 `docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md`。

- [x] **Step 2: 在 AGENTS 写入短记忆**

  记录新增部署 Runbook、8317 当前是 HTTPS/TLS，以及历史 `auth_unavailable` 根因不是 CLIProxyAPI 调度器坏。

### Task 4: 验证和结果归档

**Files:**
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-deployment-runbook-auth-unavailable-result_CN.md`

- [x] **Step 1: 运行 Markdown 差异检查**

  Run: `git diff --check`

  Expected: exit 0。

- [x] **Step 2: 粗查新增文档敏感信息**

  Run: `rg -n "sk-|HMAC|secret|password|token|api key|API Key" docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md docs/ai/context/20260718-122226-deployment-runbook-auth-unavailable-*.md AGENTS.md deploy/README.md`

  Expected: 只出现通用说明或占位符，不出现完整真实密钥。

- [x] **Step 3: 新增结果上下文**

  记录文档路径、覆盖内容、验证命令和未部署事实。

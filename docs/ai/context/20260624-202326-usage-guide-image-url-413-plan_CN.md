# 生图教程 URL 二义性与 413 归因小修计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or equivalent inline execution. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 确认当前 413 是否来自生图教程误导，并把 `/usage-guide` 生图接口说明改成不容易误填的格式。

**Architecture:** 本次只改前端用户教程文案和对应测试，不改后端路由、计费、API Key、Nginx 或公网运行配置。日志归因以 Nginx access.log/error.log 的路径、状态码和 User-Agent 为准。

**Tech Stack:** Vue 3、Vitest、Nginx access/error log。

---

## 已确认事实

- Nginx access.log 中 413 分布为 `/responses` 295 条、`/v1/responses` 19 条、`/v1/chat/completions` 2 条，未发现 `/v1/images/generations` 或 `/v1/images/edits` 的 413。
- `/responses` 413 的 User-Agent 主要是 Windows `Codex Desktop/0.142.0`，不是图片生成客户端。
- 同一 User-Agent 周期性请求 `GET /models?client_version=0.142.0` 返回 404，说明至少有客户端把 Base URL 配成了不带 `/v1` 的地址，导致 Codex Desktop 拼出裸 `/responses`、`/models`。
- `/usage-guide` 生图页面当前同时写了 `https://api.aaccx.pw/v1` 和 `POST /v1/images/edits`，对需要分别填写 Base URL 与路径的客户端存在二义性。

## Files

- Modify: `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
- Modify: `frontend/src/views/user/UsageGuideView.vue`
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-usage-guide-image-url-413-result_CN.md`
- Modify: `AGENTS.md`

## Tasks

- [ ] **Step 1: 写失败测试**

  在 `frontend/src/views/user/__tests__/UsageGuideView.spec.ts` 的“生图方法教程只展示用户需要知道的接入和扣费信息”用例中新增断言：

  ```ts
  '客户端 Base URL 填 https://api.aaccx.pw/v1',
  '接口路径填 /images/edits',
  '如果工具要求完整 URL，使用 https://api.aaccx.pw/v1/images/edits',
  ```

  并把旧的 `POST /v1/images/edits` 断言替换为完整 URL 与相对路径断言，避免继续鼓励“Base URL 带 `/v1`，路径也带 `/v1`”的组合。

- [ ] **Step 2: 运行失败测试**

  Run:

  ```bash
  pnpm --dir frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts
  ```

  Expected: 失败，提示缺少新的 Base URL / 接口路径说明。

- [ ] **Step 3: 修改生图教程文案**

  在 `frontend/src/views/user/UsageGuideView.vue`：

  - “可用范围”写清：客户端只填 Base URL 时填 `https://api.aaccx.pw/v1`。
  - “接口与扣费”写清：路径单独填写时用 `/images/edits`，完整 URL 才是 `https://api.aaccx.pw/v1/images/edits`。
  - “请求示例”保留完整 curl URL，补一句文本生图的完整 URL 为 `https://api.aaccx.pw/v1/images/generations`。

- [ ] **Step 4: 运行相关验证**

  Run:

  ```bash
  pnpm --dir frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts
  pnpm --dir frontend typecheck
  ```

  Expected: 两个命令均 exit 0。

- [ ] **Step 5: 记录结果**

  新建 `docs/ai/context/YYYYMMDD-HHMMSS-usage-guide-image-url-413-result_CN.md`，记录：

  - 413 不是图片接口集中触发；
  - `/responses` 来自 Codex Desktop 裸路径，最可能是 Base URL 少了 `/v1` 或使用了兼容裸路径客户端；
  - 已改教程降低 URL 填写二义性；
  - 本次没有改公网运行配置。

  在 `AGENTS.md` 增加一条短记忆，避免后续把裸 `/responses` 误判为图片接口问题。

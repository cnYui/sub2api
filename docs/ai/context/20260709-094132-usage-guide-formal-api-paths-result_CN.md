# /usage-guide 规范 API 请求路径栏目结果

> 2026-07-09 09:41 JST。分支：`codex/add-formal-api-usage-guide`。本轮未构建 Docker、未重启容器、未部署公网。

## 改动

- `frontend/src/views/user/UsageGuideView.vue`
  - 左侧栏目和移动端 tab 新增“规范使用”。
  - 新增“正式请求路径”表格，列出常用规范 URL、方法和大白话含义：
    - `POST https://api.aaccx.pw/v1/responses`
    - `POST https://api.aaccx.pw/v1/responses/*`
    - `POST https://api.aaccx.pw/v1/chat/completions`
    - `POST https://api.aaccx.pw/v1/embeddings`
    - `POST https://api.aaccx.pw/v1/images/generations`
    - `POST https://api.aaccx.pw/v1/images/edits`
    - `GET https://api.aaccx.pw/v1/models`
    - `GET https://api.aaccx.pw/v1/usage`
    - `POST https://api.aaccx.pw/v1/messages`
    - `POST https://api.aaccx.pw/v1/messages/count_tokens`
  - 新增“旧写法怎么改”表格，说明裸 `/responses`、`/models`、`/chat/completions`、`/embeddings`、`/images/*`、`/backend-api/codex/responses` 会返回 `400 INVALID_BASE_URL`，并给出迁移目标。
  - 新增 Codex 配置示例：`base_url = "https://api.aaccx.pw/v1"`、`wire_api = "responses"`。
  - 表格支持横向滚动，避免移动端 URL 撑破布局。
- `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
  - 补充“规范使用”栏目、关键 URL、旧路径迁移和 Codex 配置示例的回归断言。

## 验证

已先写测试并确认红灯：

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/UsageGuideView.spec.ts
```

红灯原因：页面尚未包含 `id: 'formal-api'` 和 `title: '规范使用'`。

实现后已通过：

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/UsageGuideView.spec.ts
pnpm typecheck
git diff --check
```

## 注意

- 当前分支创建时工作区已有 `AGENTS.md` 和若干 `docs/ai/context/20260709-*.md` 未提交改动；本次提交需要只纳入 `/usage-guide` 页面、对应测试、本计划/结果文档，以及本次 AGENTS 记忆。
- 本轮没有启动本地 dev server，也没有发布到 18084。

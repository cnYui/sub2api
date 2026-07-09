# /usage-guide 规范 API 请求路径栏目计划

> 2026-07-09 09:37 JST。分支：`codex/add-formal-api-usage-guide`。范围：只改 `/usage-guide` 页面、对应前端测试和本轮上下文文档；不构建 Docker、不重启服务。

## 目标

- 在 `/usage-guide` 左侧栏目新增“规范使用”。
- 用大白话说明：对外正式 Base URL 只填 `https://api.aaccx.pw/v1`，不要把 `/responses` 填进 Base URL，也不要再使用裸路径。
- 用表格列出常用规范 endpoint、请求方法和用途。
- 用表格列出旧裸路径会返回 `400 INVALID_BASE_URL`，以及应改成什么。

## 方案

- 复用 `UsageGuideView.vue` 当前 `guideTopics` 数据驱动结构，新增 `formal-api` topic。
- 给 section 增加 `endpointRows` 字段并在模板中渲染表格，避免用纯段落堆 URL。
- 新栏目放在 `Codex 接入` 后面，方便用户配置完 Key 后马上看到规范地址。
- 保持现有卡片、左侧导航、移动端 tab 交互，不新增路由。

## 验证

- 先修改 `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`，断言“规范使用”、`/v1/responses`、`400 INVALID_BASE_URL`、旧路径迁移说明等内容存在。
- 运行目标测试看到失败。
- 实现页面后重新运行目标测试。
- 运行 `pnpm typecheck` 和 `git diff --check` 做轻量验证。

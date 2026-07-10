# Dashboard 可用模型展示设计与实施计划

## 背景

用户要求删除 `/dashboard` 截图中的“按平台拆分”区域，并删除该页面上相关的后端计费展示规则。该区域当前位于 `frontend/src/components/user/dashboard/UserDashboardStats.vue`，会融合 `stats.by_platform` 与 `/user/platform-quotas` 返回的平台限额数据，展示平台用量、请求数、Token 和 quota 进度条。

用户指定不要新增后端逻辑，只把当前 API Key 可见模型硬编码展示在同一位置。已用脱敏 Key `sk-77fd...8203` 请求 `https://api.aaccx.pw/v1/models`，初始返回 10 个模型；随后用户明确要求同一区域还要展示 GPT-5.6 三款完整模型名，并删除 `gpt-image-1`、`gpt-image-1.5`、`gpt-5.4-mini`、`gpt-5.3-codex`、`gpt-5.3-codex-spark`、`codex-auto-review`、`gpt-5.2`。最终硬编码模型列表为：

- `gpt-5.6-sol`
- `gpt-5.6-terra`
- `gpt-5.6-luna`
- `gpt-5.5`
- `gpt-5.4`
- `gpt-image-2`

## 设计

采用方案 A：只修改 dashboard 前端展示。

- `DashboardView.vue` 不再请求 `/user/platform-quotas`，不再把 `platformQuotas` 传给 `UserDashboardStats`。
- `UserDashboardStats.vue` 删除“按平台拆分”与 quota/计费展示 helper，原位置改为“可用模型”卡片。
- 可用模型列表在组件内硬编码为字符串数组，不调用后端、不新增 API、不改变真实计费、认证、模型路由或 `/v1/models` 行为。
- 文案只表达“可用模型”，不展示价格、平台拆分、今日费用、请求数、Token、quota、计费规则。

## 文件边界

- 修改 `frontend/src/views/user/DashboardView.vue`：删除平台 quota 数据链。
- 修改 `frontend/src/components/user/dashboard/UserDashboardStats.vue`：删除平台拆分 UI 和 helper，新增硬编码模型列表 UI。
- 新增或修改 `frontend/src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts`：覆盖新区域展示模型、旧平台拆分和 quota 文案消失。
- 修改 `frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts`：替换或删除 dashboard 页面相关文案键，避免页面残留“按平台拆分”。
- 修改 `AGENTS.md`：记录本次设计与实现结论。

## TDD 计划

1. 新增 `UserDashboardStats` 目标测试：
   - mount 组件，传入含 `by_platform` 的 stats 和含 limit 的 `platformQuotas`。
   - 断言页面展示“可用模型”和最终 6 个硬编码模型，并断言用户要求删除的 7 个模型不再出现。
   - 断言不再展示“按平台拆分”、“今日消费”、`Claude`、`OpenAI` 平台卡片和 quota 文案。
   - 先运行该测试，预期失败，因为旧组件仍展示平台拆分。
2. 实现最小前端改动：
   - 删除 `platformQuotas` prop 使用。
   - 删除 `FusedPlatformCard`、`platformCards`、quota helper、`grayProgressBarClass` import。
   - 增加 `AVAILABLE_MODELS` 常量和模型 chip 网格。
   - 在 `DashboardView.vue` 删除 `getMyPlatformQuotas()` 导入、状态、加载函数与 `refreshAll()` 中的调用。
3. 运行目标测试，确认通过。
4. 运行前端 typecheck，确认 TS/Vue 类型通过。
5. 运行 `git diff --check`，确认无空白错误。

## 风险与取舍

- 这是静态展示，不会随运行态 `/v1/models` 自动变化；符合用户“直接硬编码展示”的要求。
- 删除 dashboard 的 platform quota 请求只影响该页面展示，不删除后端 `/user/platform-quotas` 能力，管理员配置、计费准入和实际扣费不受影响。
- 其他页面如“模型价格”仍可展示真实价格信息；本次只删除 dashboard 页面中的平台拆分和计费/quota 展示。

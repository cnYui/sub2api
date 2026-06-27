# Available Channels Price Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 隐藏普通用户侧 `/monitor` 入口，并在 `/available-channels` 页面展示当前用户可见的 GPT 5.4、GPT 5.5 和生图价格摘要。

**Architecture:** 侧边栏只修改用户导航声明，不动路由和后端接口。价格摘要在 `AvailableChannelsView.vue` 内由 `/channels/available` 响应派生，保持用户权限过滤后的真实口径。

**Tech Stack:** Vue 3、TypeScript、Vitest、Vue Test Utils、Pinia/Vue i18n mock。

---

### Task 1: 隐藏用户侧 `/monitor` 导航入口

**Files:**
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Test: `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`

- [ ] **Step 1: 写失败测试**

在 `AppSidebar.spec.ts` 中读取 `buildSelfNavItems` 函数源码，断言它不包含 `path: '/monitor'`，同时保留管理员导航里的 `/admin/channels/monitor`。

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --dir frontend test:run src/components/layout/__tests__/AppSidebar.spec.ts`

Expected: 新测试失败，因为当前用户导航仍包含 `/monitor`。

- [ ] **Step 3: 最小实现**

删除 `buildSelfNavItems` 中的用户侧 `{ path: '/monitor', label: t('nav.channelStatus') ... }` 条目。保留 `flagChannelMonitor` 常量和管理员导航，避免扩大改动。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --dir frontend test:run src/components/layout/__tests__/AppSidebar.spec.ts`

Expected: PASS。

### Task 2: 可用渠道价格摘要

**Files:**
- Modify: `frontend/src/views/user/AvailableChannelsView.vue`
- Test: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`

- [ ] **Step 1: 写失败测试**

挂载 `AvailableChannelsView.vue`，mock `@/api/channels` 返回包含 `gpt-5.4`、`gpt-5.5`、`gpt-image-2` 的可用渠道数据，mock `@/api/groups` 返回空专属倍率。断言页面文本包含 `gpt-5.4`、`gpt-5.5`、`gpt-image-2`、`$5.0000 / 1M token`、`$30.0000 / 1M token`、`$0.10 / 张`。

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --dir frontend test:run src/views/user/__tests__/AvailableChannelsView.spec.ts`

Expected: FAIL，因为页面当前没有价格摘要区域。

- [ ] **Step 3: 最小实现**

在 `AvailableChannelsView.vue` 新增 `featuredPriceItems` computed，从 `channels` 展开所有 `supported_models`，筛选模型名：`gpt-5.4`、`gpt-5.5`、以及 `billing_mode === 'image'` 或模型名包含 `image` 的生图模型。新增模板区域渲染模型名和价格行。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --dir frontend test:run src/views/user/__tests__/AvailableChannelsView.spec.ts`

Expected: PASS。

### Task 3: 综合验证与结果记录

**Files:**
- Modify: `docs/ai/context/YYYYMMDD-HHMMSS-available-channels-price-summary-monitor-hide-result_CN.md`

- [ ] **Step 1: 运行定向测试**

Run:

```bash
pnpm --dir frontend test:run src/components/layout/__tests__/AppSidebar.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts
```

Expected: PASS。

- [ ] **Step 2: 运行构建验证**

Run:

```bash
pnpm --dir frontend build
```

Expected: PASS。

- [ ] **Step 3: 新建结果文档**

在 `docs/ai/context/` 新建结果文档，记录改动文件、验证命令和结论。

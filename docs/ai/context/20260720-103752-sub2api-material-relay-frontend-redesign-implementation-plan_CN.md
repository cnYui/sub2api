# Sub2API Material Relay 全前端重设计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变业务 API、权限、支付、计费和路由行为的前提下，把 Sub2API 全前端迁移到已批准的 Material Relay 视觉与动效系统，并修复全部 12 项审计问题。

**Architecture:** 先通过 Tailwind 语义色阶、全局 motion token 和源码守卫建立统一约束，再依次重构应用壳、Overlay/Popover、Toast/进度指标、reduced-motion，最后迁移公开/认证/用户/管理核心页面。数据流和 store 保持原样，视觉责任集中在全局基础类、布局组件和少量页面容器。

**Tech Stack:** Vue 3、TypeScript、Tailwind CSS 3、Vue Transition/Teleport、Vitest、Vue Test Utils、Vite、Playwright/browser screenshots

---

## 文件职责

- `frontend/tailwind.config.js`：Material Relay 语义色阶、字体、阴影和基础动画配置。
- `frontend/src/style.css`：视觉 token、基础组件、Overlay、Drawer、meter、reduced-motion 和 pointer 媒体规则。
- `frontend/src/__tests__/motionContractSource.spec.ts`：禁止错误动效重新进入核心文件。
- `frontend/src/__tests__/visualThemeSource.spec.ts`：验证 Material Relay 语义主题，禁止旧视觉反模式。
- `frontend/src/components/layout/AppLayout.vue`、`AppSidebar.vue`、`AppHeader.vue`：应用壳与导航。
- `frontend/src/components/common/NavigationProgress.vue`、`Toast.vue`、`HelpTooltip.vue`、`DataTable.vue`：高频反馈与公共动效。
- `frontend/src/views/HomeView.vue`、`frontend/src/components/layout/AuthLayout.vue`：公开与认证入口。
- `frontend/src/views/user/PaymentView.vue`、`PaymentResultView.vue`、`DashboardView.vue`：用户核心状态和页面层级。
- `frontend/src/views/admin/DashboardView.vue`、运维指标组件：管理入口与实时数据表达。

## Task 1：建立测试护栏与 Material Relay token

**Files:**
- Create: `frontend/src/__tests__/motionContractSource.spec.ts`
- Modify: `frontend/src/__tests__/visualThemeSource.spec.ts`
- Modify: `frontend/tailwind.config.js`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: 写失败测试**

测试必须断言三条 easing、五条 duration token 存在；核心文件禁止 `transition-all`、`transition: all`、UI `ease-in`；旧黑白主题锁定改为语义 token 验证。

- [ ] **Step 2: 验证 RED**

Run: `pnpm test:run src/__tests__/motionContractSource.spec.ts src/__tests__/visualThemeSource.spec.ts`

Expected: 因 token 缺失和现有 `transition-all/ease-in` 命中而失败。

- [ ] **Step 3: 最小实现 token 与主题**

加入：

```css
:root {
  --ease-out: cubic-bezier(0.23, 1, 0.32, 1);
  --ease-in-out: cubic-bezier(0.77, 0, 0.175, 1);
  --ease-drawer: cubic-bezier(0.32, 0.72, 0, 1);
  --duration-press: 160ms;
  --duration-popover: 180ms;
  --duration-overlay-enter: 220ms;
  --duration-overlay-exit: 160ms;
  --duration-drawer: 280ms;
}
```

同步修改 Tailwind 语义色阶、系统字体、阴影和圆角。

- [ ] **Step 4: 验证 GREEN**

运行目标测试，保持只修 Task 1 涵盖的违规清单。

## Task 2：重构应用壳和导航反馈

**Files:**
- Modify: `frontend/src/components/layout/AppLayout.vue`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/components/common/NavigationProgress.vue`
- Modify: `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`
- Modify: `frontend/src/components/common/__tests__/NavigationProgress.spec.ts`

- [ ] **Step 1: 写 AppLayout/Sidebar/NavigationProgress 失败测试**
- [ ] **Step 2: 运行测试确认布局属性动画和无限导航运动导致失败**
- [ ] **Step 3: 去掉 margin/width/padding/gap/max-width 补间，只保留 drawer/label 的 transform/opacity**
- [ ] **Step 4: 把 NavigationProgress 改为静态状态条和 opacity 过渡**
- [ ] **Step 5: 给 `.btn` 增加 `scale(0.97)` press feedback，并门控 hover motion**
- [ ] **Step 6: 运行 AppSidebar、NavigationProgress、navigation integration 测试**

## Task 3：迁移 Overlay、Dropdown、Popover 和 Tooltip

**Files:**
- Modify: `frontend/src/style.css`
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/components/common/DateRangePicker.vue`
- Modify: `frontend/src/components/common/Select.vue`
- Modify: `frontend/src/components/common/ProxySelector.vue`
- Modify: `frontend/src/components/common/LocaleSwitcher.vue`
- Modify: `frontend/src/components/common/VersionBadge.vue`
- Modify: `frontend/src/components/common/SubscriptionProgressMini.vue`
- Modify: `frontend/src/components/account/AccountGroupsCell.vue`
- Modify: `frontend/src/components/common/HelpTooltip.vue`
- Modify: `frontend/src/components/common/__tests__/HelpTooltip.spec.ts`

- [ ] **Step 1: 写失败测试，禁止共享浮层的 `transition-all/ease-in/animate-scale-in`**
- [ ] **Step 2: 运行测试确认 RED**
- [ ] **Step 3: 建立 `popover-motion`、`modal-motion` 和 origin variable**
- [ ] **Step 4: 逐组件迁移 Vue Transition，保留 Teleport、Escape、click-outside**
- [ ] **Step 5: 为 HelpTooltip 增加 focus 支持和 origin-aware transition**
- [ ] **Step 6: 运行公共组件测试和 source guard**

## Task 4：迁移 Toast 与所有高频进度指标

**Files:**
- Modify: `frontend/src/components/common/Toast.vue`
- Modify: `frontend/src/components/common/__tests__/Toast.spec.ts`
- Modify: `frontend/src/views/user/KeysView.vue`
- Modify: `frontend/src/components/user/SubscriptionUsageCard.vue`
- Modify: `frontend/src/views/admin/SubscriptionsView.vue`
- Modify: `frontend/src/views/admin/ops/components/OpsConcurrencyCard.vue`
- Modify: `frontend/src/views/admin/ops/components/OpsDashboardHeader.vue`
- Modify: `frontend/src/views/admin/RiskControlView.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`

- [ ] **Step 1: 写失败测试，断言 Toast 不动画 width、不用 ease-in，meter 不用 transition-all**
- [ ] **Step 2: 运行测试确认 RED**
- [ ] **Step 3: Toast 进度改 `scaleX`，进退场改 220/160ms ease-out**
- [ ] **Step 4: 用户额度、订阅、并发、队列和设置进度改 `scaleX` 或静态更新**
- [ ] **Step 5: 健康分移除 1000ms 追赶式补间**
- [ ] **Step 6: 运行 Toast、Keys、Subscriptions、Dashboard 测试**

## Task 5：补齐 reduced-motion 并删除高频装饰运动

**Files:**
- Modify: `frontend/src/style.css`
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Modify: `frontend/src/views/admin/ops/components/OpsDashboardHeader.vue`
- Modify: `frontend/src/views/HomeView.vue`
- Modify: `frontend/src/components/layout/WorldMapBackground.vue`
- Modify: `frontend/src/components/common/LoadingSpinner.vue`
- Modify: Auth/WorldMap visual tests

- [ ] **Step 1: 写失败测试，要求关闭 smooth scroll、关键组件有 reduced-motion、自动刷新不常驻 spin**
- [ ] **Step 2: 运行测试确认 RED**
- [ ] **Step 3: 实现全局 reduced-motion、fine-pointer hover 规则**
- [ ] **Step 4: 常驻 spin/ping 改静态状态，真实请求期间才旋转**
- [ ] **Step 5: Home stagger 缩短到 30-60ms 间隔，WorldMap/Spinner 提供静态 reduced-motion 等价状态**
- [ ] **Step 6: 运行 AuthLayout、WorldMap、NavigationProgress 和 source guard 测试**

## Task 6：页面层 Material Relay 与稀有状态反馈

**Files:**
- Modify: `frontend/src/views/HomeView.vue`
- Modify: `frontend/src/components/layout/AuthLayout.vue`
- Modify: `frontend/src/views/user/PaymentView.vue`
- Modify: `frontend/src/views/user/PaymentResultView.vue`
- Modify: `frontend/src/views/user/DashboardView.vue`
- Modify: `frontend/src/components/user/dashboard/*`
- Modify: `frontend/src/views/admin/DashboardView.vue`
- Modify: `frontend/src/components/common/DataTable.vue`
- Modify: 相关测试文件

- [ ] **Step 1: 写 Payment、DataTable、主题 source guard 失败测试**
- [ ] **Step 2: 运行测试确认 RED**
- [ ] **Step 3: 实现 PaymentResult 一次性 success feedback 和 PaymentView 非阻塞阶段 transition**
- [ ] **Step 4: 实现 DataTable 排序图标 120ms 状态指示**
- [ ] **Step 5: 重做 Home/Auth/User Dashboard/Admin Dashboard 的层级、表面和响应式布局**
- [ ] **Step 6: 删除页面层高频 hover lift，改为颜色/边框或 fine-pointer 限定**
- [ ] **Step 7: 运行相关单测、typecheck 和 lint**

## Task 7：全量验证和 review-animations 复审

**Files:**
- Create: `docs/ai/context/20260720-103752-sub2api-material-relay-frontend-redesign-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 运行 `pnpm typecheck`**
- [ ] **Step 2: 运行 `pnpm lint:check`**
- [ ] **Step 3: 运行 `pnpm test:run`**
- [ ] **Step 4: 运行 `pnpm build`**
- [ ] **Step 5: 启动 Vite，截图验证 `/home`、`/login`、`/dashboard`、`/admin/dashboard` 的 `1440x900` 和 `390x844`**
- [ ] **Step 6: 用 `review-animations` 按 findings table + verdict 复审，修复所有 Block 项**
- [ ] **Step 7: 写 result 上下文，更新 AGENTS.md，并检查未跟踪 context 文档**

## 自检

- 12 项审计 finding 均映射到 Task 1-6。
- 5 条通过 Gate 的 missed opportunity 映射到 Task 2、3、6。
- 计划不改变运行态、后端、数据库、计费、支付状态机和 API 契约。
- 计划不新增动效依赖，所有预定动效使用 CSS/Vue Transition。

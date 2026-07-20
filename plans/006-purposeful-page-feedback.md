# 006 — 增加稀有状态反馈并统一页面层材质

- **Status**: TODO
- **Commit**: `7d97761d`
- **Severity**: MEDIUM
- **Category**: Missed opportunities / Cohesion
- **Estimated scope**: 16 files，约 420 行

## Problem

`PaymentResultView.vue:11-29` 成功/待处理/失败直接替换；`PaymentView.vue:9,37,76` 支付阶段整块切换；`HelpTooltip.vue:119-127` 瞬移；`DataTable.vue:97-100` 排序箭头瞬翻。同时公开页、认证页、用户 Dashboard 和管理 Dashboard 仍使用旧卡片/颜色层级，不能体现已批准的 Material Relay 设计。

## Target

- PaymentResult：稀有成功态用 `opacity + scale(0.95)`，`220ms var(--ease-out)`；失败/等待沿同一容器 crossfade，reduce 仅 opacity。
- PaymentView：阶段切换用 `opacity + translateY(4px)`，`180ms var(--ease-out)`，不得等待动画完成后才提交支付。
- DataTable 排序图标：`transform 120ms var(--ease-out)`。
- 公开/认证页和两个 Dashboard 使用同一 Material Relay 应用壳：浮动 chrome 可半透明，内容面实色，卡片圆角 `6/8px`，禁止卡片嵌套。

## Repo conventions to follow

- Home 仍显示配置化 site name/logo/subtitle/doc URL。
- AuthLayout 仍支持 worldMapBackground 变体。
- Dashboard 数据加载、轮询、Chart.js 数据和支付状态机保持不变。

## Steps

1. 扩展 `PaymentResultView.spec.ts`、`PaymentView.spec.ts`、`DataTable` source guard 和 `visualThemeSource.spec.ts`，写失败断言覆盖状态 Transition、排序 motion 和新 surface class。
2. 运行目标测试并确认失败。
3. 实现 PaymentResult/PaymentView 的非阻塞状态 transition 与 reduced-motion。
4. 实现 DataTable 排序图标和 QuickActions press feedback。
5. 重做 HomeView 与 AuthLayout 的 Material Relay 表面、排版和移动布局；删除旧 serif hero 与过度卡片化。
6. 重做用户 Dashboard 与管理 Dashboard 的统计层级、表面和响应式布局，不改数据代码。
7. 将用户/管理高频卡片 hover lift 改为颜色/边框反馈或 fine-pointer 限定。
8. 运行相关单测、typecheck 和视觉截图。

## Boundaries

- 不改变支付结果判定、轮询次数、Dashboard API 和路由。
- 不给高频路由、表格行或键盘操作增加进入动画。
- 不使用 marketing split hero、渐变背景、卡片套卡片或圆角大于 8px 的普通卡片。

## Verification

- **Mechanical**: `pnpm test:run src/views/user/__tests__/PaymentResultView.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/DashboardView.spec.ts src/views/__tests__/HomeView.spec.ts src/components/layout/__tests__/AuthLayout.spec.ts src/__tests__/visualThemeSource.spec.ts`。
- **Feel check**: 支付阶段动画不延迟操作；成功态只播放一次；四个核心页面在 `1440x900` 与 `390x844` 无重叠、无溢出、下一段内容在首屏可见。
- **Done when**: Material Relay 视觉在公开、认证、用户、管理四类入口保持一致。

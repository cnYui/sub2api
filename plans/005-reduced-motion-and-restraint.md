# 005 — 清理持续运动并补齐 reduced-motion

- **Status**: TODO
- **Commit**: `7d97761d`
- **Severity**: HIGH
- **Category**: Accessibility / Purpose & frequency
- **Estimated scope**: 10 files，约 220 行

## Problem

只有 `frontend/src/style.css:432-444` 和少数组件处理 reduced-motion。`AccountsView.vue:30` 在自动刷新启用期间常驻 spin；`OpsDashboardHeader.vue:1109` 常驻 ping；`WorldMapBackground.vue:30` 运行 75 秒全屏循环；`HomeView.vue:520-539` 将四行内容延迟到 2.5 秒；`style.css:10-12` 的平滑滚动在 reduce 下未关闭。

## Target

```css
@media (prefers-reduced-motion: reduce) {
  html { scroll-behavior: auto; }
  *, *::before, *::after {
    animation-iteration-count: 1 !important;
  }
  .motion-translate,
  .motion-scale,
  .motion-spin { transform: none !important; }
}
@media (hover: hover) and (pointer: fine) {
  .interactive-lift:hover { transform: translateY(-2px); }
}
```

高频运营状态使用静态颜色/徽标；只有真实请求期间才 spin。Home 首屏内容立即可读，若保留 stagger，间隔 `30-60ms`、单项 `160-220ms`。WorldMap 在 reduce 下静态，在普通模式也降低对比和移动幅度。

## Repo conventions to follow

- 不删除有意义的 loading/status 信息，只替换其运动表达。
- reduced-motion 保留 opacity/color 状态反馈。
- 现有 `WorldMapBackground.visual.spec.ts` 和 AuthLayout visual tests 必须继续覆盖资源与结构。

## Steps

1. 扩展 `motionContractSource.spec.ts`：失败断言全局关闭 smooth scroll、关键组件有 reduce 规则，自动刷新不因 enabled 常驻 spin。
2. 运行测试并确认失败。
3. 在 `style.css` 增加统一 reduced-motion 和 hover fine-pointer 规则。
4. 修改 Accounts/Ops 的常驻 spin/ping 为静态状态；只在真实 loading 时旋转。
5. 缩短 Home 终端 stagger，增加 reduced-motion；移除触控上的 3D hover。
6. 调整 WorldMapBackground 和 LoadingSpinner 的 reduce 行为，保留静态进度含义。
7. 运行 AuthLayout、WorldMap、NavigationProgress 与 source guard 测试。

## Boundaries

- 不删除必要的加载、错误和成功信息。
- 不在 reduced-motion 下把所有反馈清零。

## Verification

- **Mechanical**: `pnpm test:run src/__tests__/motionContractSource.spec.ts src/components/layout/__tests__/AuthLayout.visual.spec.ts src/components/layout/__tests__/WorldMapBackground.visual.spec.ts src/components/common/__tests__/NavigationProgress.spec.ts`。
- **Feel check**: 开启 reduce 后页面无滑动、缩放、循环旋转和 ping，仍能分辨加载/活动/错误状态。
- **Done when**: Chrome Rendering 面板切换 reduced-motion 时无大幅位移或持续运动。

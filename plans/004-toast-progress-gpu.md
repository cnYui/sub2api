# 004 — 让 Toast 与进度指标可中断且 GPU-only

- **Status**: TODO
- **Commit**: `7d97761d`
- **Severity**: HIGH
- **Category**: Performance / Interruptibility
- **Estimated scope**: 13 files，约 280 行

## Problem

`frontend/src/components/common/Toast.vue:8-14` 使用 `ease-in` 离场；`:179-192` 通过 keyframe 动画 `width`。`frontend/src/style.css:614-615`、`SubscriptionProgressMini.vue:81,109,137`、`KeysView.vue:151-242,603-700`、`OpsConcurrencyCard.vue:432,466-469,587-588`、`RiskControlView.vue:181` 等使用 `transition-all` + 动态宽度。`OpsDashboardHeader.vue:1075` 还用 `duration-1000` 补间实时健康分。

## Target

```css
.toast-progress {
  transform-origin: left;
  animation: toast-progress-shrink var(--toast-duration) linear forwards;
}
@keyframes toast-progress-shrink {
  from { transform: scaleX(1); }
  to { transform: scaleX(0); }
}
.meter-fill {
  transform: scaleX(var(--meter-value));
  transform-origin: left;
  transition: transform 180ms var(--ease-out);
}
```

实时轮询指标不补间；低频用户可见额度使用 `scaleX`，Toast 进入 `220ms`、退出 `160ms`，都只动画 transform/opacity。

## Repo conventions to follow

- Toast 继续消费 `useAppStore().toasts`，保留错误引用复制。
- 颜色仍由语义类型决定，倒计时 duration 继续来自 toast 数据。
- 实时运维页以数据准确和可读为先。

## Steps

1. 扩展 `Toast.spec.ts` 与 `motionContractSource.spec.ts`：失败断言 Toast 无 `ease-in`、width keyframe，meter 不含 `transition-all`。
2. 运行目标测试并确认失败。
3. 修改 Toast 进退场类与倒计时 keyframe，使用 CSS variable 传 duration。
4. 在 `style.css` 建立 `.meter-fill`；把用户额度、订阅、并发、队列和设置页进度迁移为 `scaleX` 或静态更新。
5. 将 Ops 健康分 `duration-1000 transition-all` 改为无补间或 `stroke-dashoffset 180ms var(--ease-out)`。
6. 运行 Toast、Dashboard、Keys、Subscriptions 和相关 source guard 测试。

## Boundaries

- 不改变用量、额度、健康分和计费数据计算。
- 不改变 Toast 生命周期与自动移除定时器。

## Verification

- **Mechanical**: `pnpm test:run src/components/common/__tests__/Toast.spec.ts src/views/user/__tests__/KeysView.spec.ts src/views/user/__tests__/SubscriptionsView.spec.ts src/__tests__/motionContractSource.spec.ts`。
- **Feel check**: 连续触发多个 Toast，进退场可平滑反转；15 秒 quota 刷新不产生缓慢追赶；进度条无布局抖动。
- **Done when**: DevTools 中进度更新不再触发布局动画。

# 003 — 统一 Origin-aware Overlay 与 Popover

- **Status**: TODO
- **Commit**: `7d97761d`
- **Severity**: HIGH
- **Category**: Physicality & origin / Easing & duration / Interruptibility
- **Estimated scope**: 14 files，约 320 行

## Problem

`frontend/src/style.css:336-344` 固定 `origin-top-right animate-scale-in`；`AppHeader.vue:333-341`、`DateRangePicker.vue:427-435`、`Select.vue:565-573`、`ProxySelector.vue:416-424`、`LocaleSwitcher.vue:93-101`、`VersionBadge.vue:538-546`、`SubscriptionProgressMini.vue:309-317` 都使用 `transition: all`。Modal leave 在 `style.css:403-417` 使用 `ease-in`。`AccountGroupsCell.vue:32` 同样使用 `ease-in`，且固定坐标 Popover 没有触发器来源。

## Target

```css
.popover-motion-enter-active,
.popover-motion-leave-active {
  transition: transform var(--duration-popover) var(--ease-out), opacity var(--duration-popover) var(--ease-out);
}
.popover-motion-enter-from,
.popover-motion-leave-to {
  opacity: 0;
  transform: scale(0.97) translateY(-4px);
}
```

触发器关联浮层通过 CSS variable 或组件计算设置 `transform-origin`；Modal 保持中心 origin，进入 `220ms`、退出 `160ms`，都使用 `var(--ease-out)`。HelpTooltip 增加同一套可中断 Transition，并支持 focus。

## Repo conventions to follow

- Vue `<Transition>` 继续承担 mount/unmount 生命周期。
- Teleport 和现有 click-outside/Escape 行为保持不变。
- Modal 中心 origin 是例外，不按触发器偏移。

## Steps

1. 在 `frontend/src/__tests__/motionContractSource.spec.ts` 写失败断言：核心浮层不含 `transition: all`、`ease-in` 或 `animate-scale-in`，必须引用 `popover-motion`/`modal-motion`。
2. 扩展 `frontend/src/components/common/__tests__/HelpTooltip.spec.ts`，断言键盘 focus 可显示 Tooltip，关闭后不可见。
3. 运行测试并确认失败。
4. 在 `style.css` 建立共享 popover/dropdown/modal motion 类和 origin variable。
5. 迁移 AppHeader、DateRangePicker、Select、ProxySelector、LocaleSwitcher、VersionBadge、SubscriptionProgressMini、AccountGroupsCell、AnnouncementBell、AnnouncementPopup。
6. 给 HelpTooltip 增加 125-180ms origin-aware transition；hover 只在 fine pointer 启用。
7. 运行目标测试和相关组件测试。

## Boundaries

- 不改变弹层内容、提交行为、关闭条件和 z-index 关系。
- 不动画 width/height/top/left；定位可静态计算，但动画只用 transform/opacity。

## Verification

- **Mechanical**: `pnpm test:run src/__tests__/motionContractSource.spec.ts src/components/common/__tests__/HelpTooltip.spec.ts src/components/common/__tests__/DateRangePicker.spec.ts`。
- **Feel check**: AppHeader 用户菜单、语言菜单、日期选择器、Tooltip 都从触发点出现；快速连点不会重启 keyframe；Modal 居中缩放。
- **Done when**: 所有迁移浮层没有 `transition-all/ease-in/scale(0)`，reduce 下只透明度变化。

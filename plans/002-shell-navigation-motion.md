# 002 — 重构应用壳与高频导航动效

- **Status**: TODO
- **Commit**: `7d97761d`
- **Severity**: HIGH
- **Category**: Performance / Purpose & frequency / Accessibility
- **Estimated scope**: 7 files，约 260 行

## Problem

`frontend/src/components/layout/AppLayout.vue:6` 用 `transition-all duration-300` 动画随侧栏变化的 `lg:ml-*`；`frontend/src/style.css:505-515` 动画 `width/padding/gap`；`frontend/src/components/layout/AppSidebar.vue:915-918,995-998` 动画 `max-width`。`frontend/src/components/common/NavigationProgress.vue:54-106` 每次路由导航都运行无限位移，reduced-motion 仍替换成无限 pulse。共享 `.btn` 在 `frontend/src/style.css:66-70` 没有按压反馈。

## Target

应用壳几何静态切换，只有标签和遮罩动画：

```css
.sidebar-label {
  transition: opacity 140ms var(--ease-out), transform 140ms var(--ease-out);
}
.sidebar-backdrop {
  transition: opacity 160ms var(--ease-out);
}
.btn {
  transition: transform var(--duration-press) var(--ease-out), background-color 150ms ease, border-color 150ms ease;
}
.btn:active { transform: scale(0.97); }
```

NavigationProgress 只显示静态状态条，进入/退出仅 `opacity 160ms var(--ease-out)`；reduced-motion 下无位移、循环和缩放。

## Repo conventions to follow

- 保留 `useAppStore().sidebarCollapsed/mobileOpen`、导航权限和 feature flag。
- 保留 `AppSidebar.spec.ts` 的 SVG、层级和菜单约束。
- 移动端遮罩继续保持 `header z-30 < overlay z-35 < sidebar z-40`。

## Steps

1. 扩展 `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`：失败断言 AppLayout/Sidebar 不再包含 `transition-all`、`width/padding/gap/max-width` 过渡，并要求移动遮罩使用 `sidebar-backdrop`。
2. 扩展 `frontend/src/components/common/__tests__/NavigationProgress.spec.ts`：失败断言源码不含无限 `progress-slide/progress-pulse`，reduced-motion 无持续运动。
3. 运行两个测试文件并确认预期失败。
4. 修改 `AppLayout.vue`、`AppSidebar.vue`、`AppHeader.vue` 与 `style.css`，去掉布局属性补间；保留标签 opacity/transform 和抽屉 transform。
5. 修改 `NavigationProgress.vue` 为静态状态条与透明度过渡。
6. 给共享 `.btn` 加 press feedback，并用 fine-pointer 媒体查询门控 hover 位移。
7. 运行目标测试、AppSidebar 相关测试和导航集成测试。

## Boundaries

- 不改变侧栏折叠状态持久化、权限菜单顺序、路由目标和 onboarding selector。
- 不给路由内容增加页面进入/退出动画。

## Verification

- **Mechanical**: `pnpm test:run src/components/layout/__tests__/AppSidebar.spec.ts src/components/common/__tests__/NavigationProgress.spec.ts src/__tests__/integration/navigation.spec.ts`。
- **Feel check**: 连续折叠/展开侧栏，内容区不得横向拖动 300ms；移动端遮罩与抽屉同步；键盘导航没有延迟。
- **Done when**: DevTools Performance 中折叠过程不再持续触发布局动画。

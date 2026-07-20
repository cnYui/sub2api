# 001 — 建立 Material Relay 视觉与动效 token

- **Status**: TODO
- **Commit**: `7d97761d`
- **Severity**: HIGH
- **Category**: Cohesion & tokens / Accessibility
- **Estimated scope**: 4 files，约 180 行

## Problem

`frontend/tailwind.config.js:8-92` 把 `primary`、`accent`、`dark` 固定为旧中性主题；`frontend/src/style.css:46,68,149,193,301,400-417,513-515` 分散手写 `ease`、`ease-out` 和 `0.12-0.3s`，没有共享 motion token。`frontend/src/__tests__/visualThemeSource.spec.ts:54-75` 还把旧黑白主题当作唯一合法主题。

## Target

在 `frontend/src/style.css` 根作用域定义：

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

`tailwind.config.js` 将 `primary` 改为清晰蓝色阶、`accent` 改为克制薄荷色阶、`dark` 改为中性黑灰；display 字体改为系统字体。视觉测试改为验证所有核心文件只引用语义 token，禁止重新引入 `transition-all`、旧 mesh/glow 和随机渐变。

## Repo conventions to follow

- 全局基础类仍放在 `frontend/src/style.css` 的 Tailwind layer 中。
- Tailwind 色阶继续使用 `50..950` 结构，不引入新依赖。
- 测试继续使用 Vitest + `readFileSync` 的源码守卫模式，参考 `frontend/src/__tests__/visualThemeSource.spec.ts`。

## Steps

1. 在 `frontend/src/__tests__/motionContractSource.spec.ts` 写失败测试，断言 `style.css` 包含三条 easing 和五条 duration token，且核心文件不含 `transition-all` / `transition: all`。
2. 运行 `pnpm test:run src/__tests__/motionContractSource.spec.ts`，确认因 token 缺失和现有违规而失败。
3. 修改 `frontend/tailwind.config.js` 的语义色阶、字体、阴影和动画配置；不要新增渐变背景动画。
4. 在 `frontend/src/style.css` 建立 motion/material token，并让 `.btn`、`.input`、`.card` 等共享类开始引用 token。
5. 更新 `frontend/src/__tests__/visualThemeSource.spec.ts`，从“锁死旧颜色”改为“禁止非语义颜色与视觉反模式”。
6. 运行目标测试并确认通过。

## Boundaries

- 不改 API、store、路由或业务逻辑。
- 不新增 CSS-in-JS、Motion、GSAP 等依赖。
- 不用紫蓝渐变、发光阴影或装饰性背景球。

## Verification

- **Mechanical**: `pnpm test:run src/__tests__/motionContractSource.spec.ts src/__tests__/visualThemeSource.spec.ts`，预期全部通过。
- **Feel check**: 打开 `/home`、`/dashboard` 和 `/admin/dashboard`，确认浅色/深色均为 Material Relay 语义色，正文表面保持实色高对比。
- **Done when**: 所有后续计划能只引用 token，不再手写新的 cubic-bezier。

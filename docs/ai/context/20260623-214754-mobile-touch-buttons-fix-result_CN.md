# 手机端触屏按钮无法点击修复结果

## 变更

- `frontend/src/components/layout/AppSidebar.vue`
  - 移动侧栏遮罩从 `z-30` 调整为 `z-[35]`。
  - 保持 Header 为 `z-30`、侧栏为 `z-40`，避免移动菜单打开时 Header 抢占遮罩点击，同时确保侧栏菜单项仍在遮罩之上。
- `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`
  - 新增移动遮罩层级回归测试，断言遮罩高于 Header、低于 Sidebar。

## 根因

移动菜单打开时，遮罩与 Header 同为 `z-30`，触屏命中顺序不稳定。实际复现中，菜单打开后 Header 区域的元素仍可能成为 `elementFromPoint` 的顶层元素，导致用户在手机端点击时感觉部分按钮无响应或点错层。

## 真实验证

使用用户提供的 `2799523972@qq.com` 在本地源码预览 `http://127.0.0.1:5175` 登录，后端代理到 `127.0.0.1:18080`。未记录密码。

验证结果：

- 登录成功并进入 `/dashboard`。
- 手机布局下侧栏默认隐藏，菜单按钮可见。
- 打开移动菜单后：
  - 侧栏 `z=40`。
  - 遮罩 `z=35`，class 为 `fixed inset-0 z-[35] bg-black/40 lg:hidden`。
  - Header 区域命中遮罩，不再命中 Header 内按钮。
  - 侧栏「API 密钥」链接中心点命中自身，可点击。
- 点击侧栏「API 密钥」后进入 `/keys`，遮罩消失。
- `/keys` 页面「创建密钥」按钮和右上角用户菜单中心点均命中自身或子元素。
- 用户下拉菜单打开后，dropdown `z=50`，下拉里的「API 密钥」菜单项中心点命中自身。

## 验证命令

- `pnpm test:run src/components/layout/__tests__/AppSidebar.spec.ts`
- `pnpm typecheck`
- `pnpm build`

说明：验证过程中发现 `frontend/node_modules` 一度缺失，已用 `pnpm install --frozen-lockfile` 按 lockfile 恢复依赖后重新执行上述验证。

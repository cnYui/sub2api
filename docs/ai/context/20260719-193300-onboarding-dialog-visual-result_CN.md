# 新手引导弹窗视觉统一结果

## 已完成

- `frontend/src/styles/onboarding.css` 改为站内黑白灰体系：亮色白底黑字、深色近黑底白字、灰阶边框与轻阴影，主按钮使用黑白反色。
- 弹窗字体改为与全站一致的 Inter 和中文系统字体回退；圆角、按钮、快捷键、高亮边框均采用现有后台的紧凑样式。
- 新增 `normalizeOnboardingSteps`，在步骤传入 Driver.js 前统一清理标题、说明和按钮文案中的 emoji，同时保留原有 HTML 结构与多语言文案。
- 对说明中的内联提示框覆盖为灰阶样式，避免遗留红色、蓝绿等局部配色。
- 为 emoji 清理和引导灰阶样式新增回归测试。

## 验证

- `pnpm exec vitest run src/utils/__tests__/onboardingContent.spec.ts src/__tests__/visualThemeSource.spec.ts`：3 项通过。
- `pnpm typecheck`：通过。
- `pnpm build`：通过；仅有项目原有的分包和 Browserslist 数据提示。
- 本地 `sub2api-dev` 已按原 Compose 项目重建，`http://127.0.0.1:8080/health` 与 `/login` 均返回 200，PostgreSQL、Redis 和 CLIProxyAPI 未重启。

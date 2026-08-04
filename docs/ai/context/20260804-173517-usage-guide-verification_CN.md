# 使用方法页面迁移验证

## 验证范围

验证已迁移的 `/usage-guide` 认证页面、侧栏入口、多语言文案、教程媒体资源和响应式展示。

## 自动化验证

- `pnpm exec vitest run src/views/user/__tests__/UsageGuideView.spec.ts`：3 个测试全部通过。
- `pnpm run typecheck`：通过。
- `pnpm run build`：通过；仅保留项目已有的 Browserslist、动态分包和大产物提示。
- 本次提交范围执行 `git diff --check`：通过。

## 运行时检查

- 使用已登录用户访问 `http://localhost:3003/usage-guide`，确认侧栏“使用方法”入口及认证路由可用。
- 桌面端显示左侧主题导航，移动端显示可横向滚动的主题标签；页面内容容器未产生横向溢出。
- 七个教程主题、13 张步骤截图、CCSwitch 视频和视频封面均已加载；视频 `readyState=4` 且无媒体错误。
- 浅色与深色模式下卡片、表格、代码块和正文均可读。

## 边界

宽屏中检测到的 26px 文档横向宽度来自既有 `AppHeader` 用户菜单按钮，不由使用方法页面的内容容器造成。

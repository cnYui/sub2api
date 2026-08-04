# 18080 使用方法页面迁移

## 背景

当前项目缺少旧 18080 项目用户侧的“使用方法”栏目。旧栏目不是单一教程，而是包含 Codex 接入、CCSwitch 视频、规范 API、VS Code Copilot、生图、错误编号和 Trae 接入的完整页面，并依赖 13 张步骤截图、1 个视频和 1 张视频封面。

## 决策

- 保留旧页面的全部七个主题、步骤、表格、代码示例和本地媒体，不缩减业务内容。
- 路由继续使用认证用户页面 `/usage-guide`，侧栏入口放在兑换之后，普通用户和管理员的“我的账户”区域复用同一条导航声明。
- 页面字体继承当前应用全局字体，不单独加载旧字体；颜色沿用当前项目的灰阶、深色模式和蓝色焦点色。
- 卡片、标签、表格、代码块和图片统一采用当前项目的圆角、边框、阴影和响应式断点，并为低动态偏好关闭非必要过渡。
- 页面正文保持旧 18080 项目的中文内容；路由标题和侧栏入口补齐中英文国际化，避免英文界面出现缺失键。

## 实现范围

- 页面：`frontend/src/views/user/UsageGuideView.vue`
- 路由：`frontend/src/router/index.ts`
- 导航：`frontend/src/components/layout/AppSidebar.vue`
- 国际化：`frontend/src/i18n/locales/{zh,en}/common.ts`、`landing.ts`
- 图片：`frontend/src/assets/usage-guide/`
- 视频：`frontend/public/usage-guide/`
- 验证：`frontend/src/views/user/__tests__/UsageGuideView.spec.ts`

## 验证口径

- 七个主题及桌面/移动端主题导航必须存在。
- 13 张截图、视频和封面必须随构建产物提供。
- `/usage-guide` 必须要求登录且可从侧栏进入。
- 前端测试、类型检查和生产构建必须通过。

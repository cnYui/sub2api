# Claude Code 桌面端使用方法页面补充

## 需求

在现有认证用户 `/usage-guide` 页面中补充 Claude Code 桌面端接入教程，使用用户提供的 6 张 CC Switch 和 Claude Code 截图。

## 实现决策

- 复用 `UsageGuideView.vue` 已有的“主题 - 步骤 - 图片”数据结构和 `guide-step` 图片渲染组件，不新增平行页面或图片组件。
- 新增“Claude Code 桌面端接入”主题，共 3 步：切换 Cloud 分组、在 CC Switch 中添加 Cloud Desktop 供应商、填写配置并获取模型后启用路由和重启桌面端。
- 第 3 步按用户给出的顺序展示 4 张连续截图，保持操作链路完整；图片统一存放在 `frontend/src/assets/usage-guide/` 并通过静态导入参与构建。
- API Key 和请求地址仅作为截图中的配置字段展示，教程文字不写入任何真实密钥。

## 变更范围

- 页面：`frontend/src/views/user/UsageGuideView.vue`
- 测试：`frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
- 图片：`frontend/src/assets/usage-guide/claude-code-step-*.png`

## 验证口径

- 使用方法页面源码包含新主题及 6 张图片引用。
- 6 张图片资源全部存在，并由现有图片组件渲染。
- `npm run test:run -- src/views/user/__tests__/UsageGuideView.spec.ts`：3 个测试全部通过。
- `npm run typecheck`：通过。
- `npm run lint:check -- src/views/user/UsageGuideView.vue src/views/user/__tests__/UsageGuideView.spec.ts`：通过。
- `npm run build`：通过，Vite 构建产物包含 6 张 Claude Code 截图资源；仅保留项目既有的 Browserslist、动态导入和大 chunk 警告。
- 本地 Vite 开发服务因 `5173-5175` 已占用，自动使用 `http://127.0.0.1:5176/usage-guide`；HTTP 状态为 200。

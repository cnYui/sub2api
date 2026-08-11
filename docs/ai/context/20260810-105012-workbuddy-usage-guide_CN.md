# WorkBuddy 使用方法更新

## 背景

按用户提供的 WorkBuddy 截图，在登录用户的 `/usage-guide` 页面增加外部模型接入方法。

## 实现

- 新增主题 `workbuddy`，更新时间为 `2026-08-10`，按现有步骤教程组件展示。
- 四步内容依次为：右下角添加自定义模型、选择服务商列表底部的 `Custom`、填写 `https://api.aaccx.pw/v1`/API Key/准确模型名、保存后开始对话。
- 在第三步明确模型名称必须以当前 API Key 的 `/v1/models` 返回值为准，示例使用 `gpt-5.5`；截图中的 API Key 已保持遮罩。
- 新增资源：`frontend/src/assets/usage-guide/workbuddy-step-01-add-custom-model.png` 至 `workbuddy-step-04-start-chat.png`。
- 更新 `UsageGuideView` 定向测试，覆盖主题标识、日期、关键配置文案和四张资源存在性。

## 验证

- `pnpm exec vitest run src/views/user/__tests__/UsageGuideView.spec.ts`：3 个测试全部通过。
- `pnpm exec vue-tsc --noEmit`：通过。
- `pnpm run build`：通过；构建输出包含四张 WorkBuddy 截图，保留仓库已有的 chunk、Browserslist 和动态导入警告。
- 浏览器访问本地 `/usage-guide` 时被登录守卫重定向到 `/login`，当前浏览器无登录态，因此未完成登录后页面截图核对。

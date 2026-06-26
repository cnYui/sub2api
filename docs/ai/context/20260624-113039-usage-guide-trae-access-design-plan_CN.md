# Usage Guide Trae 接入设计与计划

## 目标

在普通用户 `/usage-guide` 页面新增「Trae 接入」方法，让用户可以把 Sub2API 的 API Key 配置到 Trae 中使用。

## 设计

- 复用现有 `UsageGuideView.vue` 的 topic 数据结构和 `kind: 'steps'` 渲染样式。
- 左侧/移动端栏目在「Codex 接入」「生图方法」之外新增「Trae 接入」。
- 右侧说明展示 4 个步骤：
  1. 点击“添加模型”
  2. 选择“自定义配置”
  3. 填入 `https://api.aaccx.pw/v1`、自己的 API Key 和 `gpt-5.5`
  4. 在“自定义模型”中点击 `gpt-5.5` 即可使用
- 图片资源放入 `frontend/src/assets/usage-guide/`，文件命名使用 `trae-step-*.png`。

## 取舍

- 不新建路由：新增方法属于使用说明的一部分，独立页面会扩大导航和维护面。
- 不做纯文字 `sections`：用户提供了完整过程截图，用 step 样式更符合现有页面。
- 不展示真实 API Key：文案只写“自己的 API Key”，避免文档和截图泄露密钥。

## 实施计划

1. 在 `frontend/src/views/user/__tests__/UsageGuideView.spec.ts` 增加失败测试，断言新增 Trae 栏目、4 个步骤、4 张截图、URL 与 `gpt-5.5`。
2. 运行该测试，确认因缺少 Trae 内容失败。
3. 复制用户提供的 4 张 Trae 截图到 `frontend/src/assets/usage-guide/`。
4. 在 `frontend/src/views/user/UsageGuideView.vue` 导入图片并新增 `traeSetupSteps` 与 `guideTopics` topic。
5. 运行 `UsageGuideView.spec.ts`、前端 typecheck 和 build。
6. 新增结果文档，并在 `AGENTS.md` 运行记录追加本次上下文。

## 验证命令

```bash
pnpm --dir frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts
pnpm --dir frontend typecheck
pnpm --dir frontend build
```

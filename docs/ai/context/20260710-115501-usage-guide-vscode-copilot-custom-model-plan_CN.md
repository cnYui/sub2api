# /usage-guide VS Code Copilot 自定义模型接入说明计划

## 目标

- 在用户侧 `/usage-guide` 页面新增 VS Code Copilot 自定义模型接入说明。
- 内容覆盖通过 `https://api.aaccx.pw/v1` 和用户自己的 API Key 接入 Copilot 的文件修改方法。
- 说明 `gpt-5.5` 的思考程度配置，重点包含 `xhigh`。
- 不把任何真实 API Key 写入前端源码、测试或文档，只使用 `sk-xxxx` 占位符。

## 设计

- 在 `frontend/src/views/user/UsageGuideView.vue` 新增一个 `sections` 类型栏目：
  - `id: 'copilot-vscode'`
  - `title: 'VS Code Copilot 接入'`
  - 放在“规范使用”之后，便于先理解 `/v1` 正式入口，再看 Copilot 专项配置。
- 新增代码示例常量，展示需要修改的两个 VS Code 配置文件：
  - `~/Library/Application Support/Code/User/chatLanguageModels.json`
  - `~/Library/Application Support/Code/User/profiles/builtin/agents/chatLanguageModels.json`
- 示例使用 `customendpoint`、`apiType: "responses"`、`url: "https://api.aaccx.pw/v1/responses"`。
- 每个模型显式写 `requestHeaders.Authorization: "Bearer sk-xxxx"`，避免 Copilot 运行时没有把顶层 `apiKey` 合并到请求头。
- 给模型声明：
  - `model: "gpt-5.5"`
  - `toolCalling: true`
  - `supportsReasoningEffort`
  - `reasoningEffortFormat: "openai"`
  - `supportedReasoningEfforts: ["minimal", "low", "medium", "high", "xhigh"]`
  - `zeroDataRetentionEnabled: true`
- 明确 `xhigh` 对应 Copilot UI 里的 `Extra High`，如果 `medium` 可用但 `xhigh` 报 `previous_response_id`，需要确认配置里有 `zeroDataRetentionEnabled: true` 并执行 `Developer: Reload Window` 后新开会话。

## 测试计划

- 先更新 `UsageGuideView.spec.ts`，断言新增栏目、关键路径、配置字段、思考程度和敏感信息保护。
- 运行目标 vitest，确认红灯。
- 实现页面内容。
- 运行：
  - `cd frontend && pnpm vitest run src/views/user/__tests__/UsageGuideView.spec.ts`
  - `cd frontend && pnpm typecheck`
  - `git diff --check`

## 边界

- 不修改后端、计费、真实运行态配置或 VS Code 本机配置。
- 不新增截图资源。
- 不写真实本地 Key。

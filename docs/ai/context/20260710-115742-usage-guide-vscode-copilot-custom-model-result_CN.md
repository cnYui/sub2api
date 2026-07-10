# /usage-guide VS Code Copilot 自定义模型接入说明结果

## 改动

- `frontend/src/views/user/UsageGuideView.vue`
  - 新增“VS Code Copilot 接入”栏目。
  - 说明需要同步修改两个 VS Code 配置文件：
    - `~/Library/Application Support/Code/User/chatLanguageModels.json`
    - `~/Library/Application Support/Code/User/profiles/builtin/agents/chatLanguageModels.json`
  - 给出 `customendpoint` + `responses` + `https://api.aaccx.pw/v1/responses` 的配置示例。
  - 示例显式写 `requestHeaders.Authorization: Bearer sk-xxxx`，避免 Copilot 未带 Authorization header。
  - 说明 `gpt-5.5` 的 `toolCalling`、`supportsReasoningEffort`、`reasoningEffortFormat`、`supportedReasoningEfforts` 和 `zeroDataRetentionEnabled`。
  - 说明 `xhigh` 对应 Copilot UI 的 `Extra High`，以及 `medium` 可用但 `xhigh` 报 `previous_response_id` 时应检查 `zeroDataRetentionEnabled: true` 并执行 `Developer: Reload Window`。
- `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
  - 将教程栏目断言更新为五个栏目。
  - 新增 VS Code Copilot 接入栏目关键内容断言。
  - 保留真实本地 Key 不得出现在页面源码中的敏感信息保护断言。

## 验证

已先写测试并确认红灯：

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/UsageGuideView.spec.ts
```

红灯原因：页面尚未包含 `id: 'copilot-vscode'` 和 `title: 'VS Code Copilot 接入'`。

实现后已通过：

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/UsageGuideView.spec.ts
pnpm typecheck
cd ..
git diff --check
```

## 注意

- 本轮没有修改后端、数据库、运行态 VS Code 配置、Docker 或公网 18084。
- 前端页面和上下文文档没有写入真实 API Key，只保留 `sk-xxxx` 占位符。
- 工作区原本已有多项后端未提交改动，本轮未触碰。

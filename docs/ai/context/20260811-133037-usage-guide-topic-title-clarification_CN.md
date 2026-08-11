# `/usage-guide` 教程标题范围澄清

## 变更

为减少教程名称对可用模型范围的误导，更新 `frontend/src/views/user/UsageGuideView.vue` 中四个主题标题：

- `WorkBuddy 接入` → `WorkBuddy 接入中转站所有模型`
- `Codex 接入` → `Codex 接入中转站除GPT模型以外的外部模型`
- `VS Code Copilot 接入` → `VS Code Copilot 接入中转站所有模型`
- `Trae 接入` → `Trae 接入中转站所有模型`

同步在 `UsageGuideView.spec.ts` 增加四个完整标题断言。教程步骤、路由和资源未修改。

## 验证

执行 `pnpm exec vitest run src/views/user/__tests__/UsageGuideView.spec.ts`（在 `frontend` 目录）和 `pnpm typecheck`，均通过；`git diff --check` 通过。

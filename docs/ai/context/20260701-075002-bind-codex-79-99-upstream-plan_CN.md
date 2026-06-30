# 79/99 元套餐上游绑定本地代码实现计划

时间：2026-07-01 07:50 JST

## 修改范围

- 新增脚本：`scripts/bind-codex-subscription-upstreams.mjs`
- 新增脚本测试：`scripts/__tests__/bind-codex-subscription-upstreams.test.mjs`
- 修改文案：`frontend/src/views/user/UsageGuideView.vue`
- 修改测试：`frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
- 新增结果文档：`docs/ai/context/YYYYMMDD-HHMMSS-bind-codex-79-99-upstream-result_CN.md`

## 步骤

1. 写脚本测试，先让 `node --test scripts/__tests__/bind-codex-subscription-upstreams.test.mjs` 因找不到模块失败。
2. 实现脚本导出函数：
   - `parseArgs`
   - `buildRuntimeSql`
   - `buildSummary`
   - `runCli`
3. 让脚本测试通过。
4. 修改 usage guide 文案和测试断言。
5. 运行：
   - `node --test scripts/__tests__/bind-codex-subscription-upstreams.test.mjs`
   - `pnpm --dir frontend vitest run src/views/user/__tests__/UsageGuideView.spec.ts`
6. 运行 `git diff --check`。
7. 写结果上下文文档。

## 执行后运行态发布建议

本地代码合并后，公网执行顺序应为：

1. 备份 `sub2api-candidate-postgres`。
2. 确认 18084 已应用 156/157 迁移，或先发布本地 main 让迁移补齐 79。
3. 执行：

```bash
node scripts/bind-codex-subscription-upstreams.mjs \
  --pg-container=sub2api-candidate-postgres \
  --account-name=cliproxy-local-openai \
  --apply
```

4. 验证 `account_groups` 中 `codex-pool-69-usd` 和 `codex-pool-89-usd` 都绑定到 `cliproxy-local-openai`。
5. 使用 99 元用户 Key 请求 `gpt-5.4-mini`，确认 HTTP 200 并产生 `usage_logs`。

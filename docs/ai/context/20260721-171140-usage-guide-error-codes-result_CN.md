# /usage-guide 错误编号参考结果

## 变更内容

- 从本地 `main` 创建分支 `codex/usage-guide-error-codes`。
- 在 `/usage-guide` 页面新增“错误编号参考”栏目。
- 复用现有使用指南卡片和表格样式，新增 `errorRows` 表格类型。
- 依据 `docs/ERROR_CONTRACT.md` 当前 catalog 完整展示 `S2A-xxxx` 编号、英文机器码、HTTP/事件类型和规范英文提示。
- 页面说明标准响应头 `X-Sub2API-Error-ID`、`X-Sub2API-Error-Code`、`X-Sub2API-Retryable`、`Retry-After`，并提示兼容响应中的 `error_id`、`sub2api_code`、`retryable`、`retry_after`、`request_id`。

## 测试

- 先新增 `UsageGuideView.spec.ts` 断言，运行目标测试确认失败：缺少 `id: 'error-codes'`。
- 补页面实现后运行 `pnpm exec vitest run src/views/user/__tests__/UsageGuideView.spec.ts`，11 个测试通过。
- 运行 `pnpm typecheck` 通过。
- 运行 `pnpm lint:check` 通过。

## 边界

- 本次只修改前端 `/usage-guide` 页面和对应测试，不修改后端错误契约、运行态或部署配置。
- 工作树中原本存在未跟踪文档 `docs/ai/context/20260721-165708-billing-quota-usage-image-plan_CN.md`，与本次错误编号页面无关，本分支不纳入。

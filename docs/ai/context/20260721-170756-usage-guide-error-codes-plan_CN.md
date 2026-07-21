# /usage-guide 错误编号参考计划

## 背景

用户希望在已登录用户页面 `/usage-guide` 中补充 Sub2API 错误编号参考，把错误契约里的 `S2A-xxxx` 序号、英文机器码、HTTP 状态和规范英文提示展示给用户，作为后续统一报错规范。

## 必须解决的问题

- 页面需要沿用现有使用指南的栏目、卡片和表格样式，不引入新的页面路由。
- 数据来源以 `docs/ERROR_CONTRACT.md` 当前 catalog 为准。
- 英文提示必须保持规范原文，避免前端另写一套不一致文案。
- 新增内容需要有前端测试覆盖，确认 `/usage-guide` 声明该栏目并包含关键错误编号。

## 实施方案

- 在 `UsageGuideView.spec.ts` 先增加失败测试，要求页面包含“错误编号参考”栏目和代表性错误编号。
- 在 `UsageGuideView.vue` 增加 `GuideErrorRow` 类型和 `errorRows` 渲染分支，复用现有 `.usage-guide-endpoint-table` 样式。
- 新增 `errorCatalogRows` 常量，录入 `docs/ERROR_CONTRACT.md` 的完整 catalog。
- 在 `guideTopics` 中新增 `error-codes` sections 栏目。

## 验证

- 先运行目标测试并确认新增断言在生产代码未实现时失败。
- 修改页面后运行 `pnpm exec vitest run src/views/user/__tests__/UsageGuideView.spec.ts`。
- 最后运行前端 `pnpm typecheck`。

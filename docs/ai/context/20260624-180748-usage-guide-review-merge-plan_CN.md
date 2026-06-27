# UsageGuide 恢复校验与主分支合并计划

## 范围

- 只处理 UsageGuide 恢复与合并相关内容。
- 不处理当前分支上与部署脚本相关的历史提交。

## 执行步骤

1. 补测试红灯
   - 修改 `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
   - 让测试明确要求：
     - 页面包含 `Trae 接入`
     - `Trae` 四张截图说明存在
     - 14 张截图资源文件存在
2. 运行单测，确认当前版本失败
   - 目标命令：`cd frontend && npx vitest run src/views/user/__tests__/UsageGuideView.spec.ts`
3. 修复实现
   - 修改 `frontend/src/views/user/UsageGuideView.vue`
   - 恢复 `guideTopics` 中的 `Trae 接入`
   - 保留当前“图生图 + 99 元套餐”说明
   - 从历史提交恢复 `frontend/src/assets/usage-guide/trae-step-*.png`
4. 重新验证
   - `cd frontend && npx vitest run src/views/user/__tests__/UsageGuideView.spec.ts src/views/user/__tests__/PaymentView.spec.ts`
   - `cd frontend && npm run typecheck`
   - `cd frontend && npm run build`
5. 本地清理
   - 删除仓库根目录下这批临时备份文件：
     - `.tmp-*`
     - `*.dump`
     - `*.sqlite*`
   - 再检查 `git status --short`
6. 提交与合并
   - 在当前分支只提交 UsageGuide 相关文件
   - 记录该提交 SHA
   - 切换到本地 `main`
   - `git cherry-pick <sha>`
   - 在 `main` 上复跑关键验证
7. 最终复核
   - `git status --short --branch`
   - 说明当前 worktree 是否干净、当前所在分支是否为 `main`

## 风险控制

- 不直接 merge 当前分支，避免把部署脚本提交误带入 `main`。
- 不回退用户未授权的其他文件。
- 只有在测试、typecheck、build 都重新通过后才允许进入合并步骤。

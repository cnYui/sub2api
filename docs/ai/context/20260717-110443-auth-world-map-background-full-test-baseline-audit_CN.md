# 登录与注册页世界地图背景全量测试基线审计

## 结论

功能分支 `codex/auth-world-map-background` 的完整前端测试套件未全绿，但全部 11 个失败均可在主工作区未改动的前端代码上复现，且本功能分支没有修改相关实现或测试文件。因此这些失败属于当前仓库基线，不是世界地图背景改动引入的回归。

## 功能分支全量结果

命令：

```bash
cd frontend
npm run test:run
```

结果：

- 152 个测试文件：146 通过，6 失败。
- 891 个测试：880 通过，11 失败。
- exit code 1。

失败文件：

- `src/views/user/__tests__/UsageView.spec.ts`：3 个失败。
- `src/components/admin/usage/__tests__/UsageTable.spec.ts`：1 个失败。
- `src/components/charts/__tests__/ModelDistributionChart.spec.ts`：3 个失败。
- `src/components/charts/__tests__/GroupDistributionChart.spec.ts`：2 个失败。
- `src/components/payment/__tests__/PaymentQRDialog.spec.ts`：1 个失败。
- `src/composables/__tests__/usePersistedPageSize.spec.ts`：1 个失败。

## 基线复现

主工作区在验证期间已处于另一个用户工作分支 `codex/code-redundancy-refactor`，存在 AGENTS、后端测试和新上下文文档改动。本次验证没有切换分支、修改文件或暂存任何内容，只在其未改动的前端代码上运行相同 6 个失败测试文件。

结果：

- 6 个测试文件全部失败。
- 21 个测试中 10 通过、11 失败。
- 失败测试名称、断言和错误栈与功能分支一致。
- exit code 1。

同时执行：

```bash
git diff cfa0b548d..HEAD -- <上述失败实现和测试文件>
```

没有差异，证明世界地图背景提交未触及失败范围。

## 本功能独立验证

- 世界地图背景目标测试：4 个文件、10 个测试通过。
- `npm run typecheck`：exit code 0。
- `npm run build`：exit code 0。
- 目标文件 ESLint：exit code 0。
- 页面 DOM、动画位移、水平溢出、移动布局和非目标页面范围验证通过。

## 收尾边界

- 按 `finishing-a-development-branch` 规则，完整测试套件非绿时不自动合并。
- 保留 worktree `/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-auth-world-map-background` 和分支 `codex/auth-world-map-background`。
- 不修复与本次视觉需求无关的用量、图表、支付和本地存储测试。
- 未推送远端、未创建 PR、未修改运行态。

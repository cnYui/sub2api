# Available Channels 页面收敛为模型价格页结果

## 本次改动

- `.gitignore`
  - 新增 `.worktrees/` 忽略规则，避免隔离工作区目录污染仓库状态。
- `frontend/src/views/user/AvailableChannelsView.vue`
  - 移除搜索框、刷新按钮、渠道表和表格布局依赖。
  - 页面主体收敛为单一价格卡区域，只保留模型价格与生图价格展示。
  - 保留现有价格数据加载链路：`/channels/available`、`/channels/prices`、`/groups/available`。
- `frontend/src/i18n/locales/zh.ts`
  - 页面标题改为 `模型价格`
  - 页面描述改为 `查看当前账号可用模型与生图价格`
- `frontend/src/i18n/locales/en.ts`
  - 页面标题改为 `Model Prices`
  - 页面描述改为 `View model and image generation prices available to this account`
- `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`
  - 更新测试断言：保留价格卡校验，并确认页面不再渲染搜索框。

## 验证

已在隔离 worktree `/Users/wujianxiang/CodeSpace/sub2api/.worktrees/model-prices-available-channels-20260624` 执行：

```bash
pnpm -C frontend test:run src/views/user/__tests__/AvailableChannelsView.spec.ts
pnpm -C frontend build
```

结果：

- 目标测试通过：`1` 个文件，`2` 个测试全部通过。
- 前端构建通过。
- `vite build` 过程中仍有仓库原有的 chunk size / dynamic import 警告，但不影响本次改动通过构建。

## 说明

- 这次为了避开原工作树中大量未提交改动，改动发生在独立 worktree 分支 `codex/model-prices-available-channels-20260624`。
- `AGENTS.md` 未在该 worktree 内追加运行记录，原因是原工作树中的 `AGENTS.md` 已存在未提交差异，直接修改会增加后续合并冲突风险。

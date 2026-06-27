# UsageGuide 恢复校验与主分支合并结果

## 结果

- 已完成对本次 `UsageGuide` 恢复改动的 review。
- 已修复发现的阻塞问题：
  - 恢复 `Trae 接入` 栏目
  - 恢复 4 张 `Trae` 教程截图
  - 保留“使用方法 2”要求的图生图说明
  - 生图说明已包含 `29/39/59/99` 元套餐
  - 解决合并到 `main` 时产生的本地化重复 key
- 已将最终改动合并到当前工作区的本地 `main`。

## review 结论

### 已修复的阻塞问题

1. `UsageGuideView.vue` 恢复不完整
   - 文件中曾缺少 `Trae 接入` 主题挂载
   - 同时缺少 `trae-step-*.png` 资源
   - 会导致页面功能缺失，并存在前端构建风险

2. 测试覆盖不足
   - 原测试没有约束 `Trae 接入`
   - 原测试没有校验 14 张教程截图资源是否都存在
   - 现已补齐

3. 合并到 `main` 后的重复本地化 key
   - `frontend/src/i18n/locales/en.ts`
   - `frontend/src/i18n/locales/zh.ts`
   - 现已去重

### 当前无新的阻塞问题

- `main` 当前 HEAD 已是本次修复后的提交：`4e5e4c9e`
- 当前工作区 `git status --short --branch` 结果为：
  - `## main...origin/main [ahead 39]`
  - 无未提交文件

## 验证

在最终 `main` 工作树上重新执行并通过：

1. `cd frontend && npx vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/UsageGuideView.spec.ts`
2. `cd frontend && npm run typecheck`
3. `cd frontend && npm run build`

说明：

- `build` 过程中仍有既有的 Vite chunk / dynamic import warning，但命令退出码为 `0`，不属于本次改动新引入的问题。

## 本地清理

- 已将这批本地备份文件加入 `.git/info/exclude`，不再污染 `git status`：
  - `.tmp-*`
  - `*.dump`
  - `*.sqlite`
  - `*.sqlite-shm`
  - `*.sqlite-wal`

这一步是本地仓库配置，不会进入提交，也不会影响远端仓库内容。

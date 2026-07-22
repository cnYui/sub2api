# /admin/groups 与 /admin/subscriptions 分组名称显示调整结果

## 改动

- 新增前端公共展示名工具：公共 Codex 订阅分组按人民币套餐名展示。
- `/admin/groups`：
  - 表格名称列改为 `29/39/59/79/99/149/199 元订阅池`。
  - 排序弹窗、复制账号下拉、兜底分组下拉和删除确认同步使用同一展示名。
- `/admin/subscriptions`：
  - 订阅列表的分组 badge 改为人民币套餐名。
  - 顶部分组筛选和手动分配订阅下拉同步使用同一展示名。

## 边界

- 未修改数据库 `groups.name`，仍保留 `codex-pool-xx-usd` 内部标识。
- 非公共 Codex 订阅组没有人民币套餐价，继续显示原始组名。
- 本次未改变后端搜索逻辑；分组搜索仍按内部 name/description。

## 验证

- `pnpm --dir frontend exec vitest run src/utils/__tests__/subscriptionQuota.spec.ts`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`

以上均通过。

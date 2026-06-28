# 79 元订阅套餐上架结果

## 变更

- 已从本地 `main` 切出分支 `codex/add-79-subscription-plan`。
- 新增迁移 `backend/migrations/156_seed_codex_79_subscription_plan.sql`。
- 新增或更新 `codex-pool-69-usd` 分组，日限额为 69 USD。
- 新增或更新 `79 元订阅池`，售价 79.79 元，30 天有效期，`for_sale=true`，排序为 79。
- 扩展 `/purchase` 前端测试 fixture，覆盖 29/39/59/79/99 五个套餐。
- 更新 `AGENTS.md`，记录 79.79 元套餐与 `codex-pool-69-usd` 的长期约定。

## 设计约束

- 不改支付 provider 配置。
- 不复制支付和履约逻辑。
- 不绑定上游账号；迁移不包含 `account_groups` 写入。
- `/admin/orders/plans` 和 `/purchase` 都继续读取后端 `subscription_plans`，因此 79 套餐通过同一数据源展示。

## 验证

- 已先让后端迁移测试因缺少 156 文件失败，再补迁移并通过。
- 已先让 `/purchase` 测试因只有 4 个套餐失败，再补 79 fixture 并通过。
- 最终验证命令：
  - `go test -count=1 -tags=unit ./migrations`
  - `npm test -- --run src/views/user/__tests__/PaymentView.spec.ts`

## 部署提醒

- 运行态 18084 当前数据库已经应用到 191 个迁移；新迁移 156 需要随新镜像启动或手动执行迁移后才会写入公网库。
- 迁移只会 seed 数据，不会重启容器、不改 SMTP、不改支付商户密钥。

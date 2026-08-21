# DeepSeek 分组倍率恢复为 3.5x

## 变更内容

- 执行时间：2026-08-21 10:40（Asia/Tokyo）。
- 按管理员最新要求，将生产 `groups.id=8` 的 DeepSeek 分组倍率从 `0.5000` 恢复为 `3.5000`。
- 分组名称仍为 `【国产】DeepSeek（5折）`，官方基础价未修改。
- 恢复后的模型广场实付价格：
  - `deepseek-v4-pro`：`$2.31 / $6.93 / $0.077`。
  - `deepseek-v4-flash`：`$0.77 / $2.31 / $0.0245`。
- 未修改历史用量、订单、余额、用户专属倍率、账号统计倍率或最终计费倍率。

## 验证结果

- PostgreSQL 事务更新成功，既有 `trg_groups_auth_cache_invalidation` 触发器正常触发。
- 后端定向测试通过：`go test -tags unit ./internal/service -run 'TestGetModelPricing_DeepSeekUsesOfficialFallbackOverDynamicPrice|TestListPlazaGroups_DeepSeekUsesOfficialPrice' -count=1`。
- 前端模型广场测试通过：`pnpm exec vitest run src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts`，14 项全部通过。
- 本地与公网模型广场接口均返回分组倍率 `3.5` 及上述实付价格。

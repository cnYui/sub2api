# DeepSeek 分组倍率调整为 0.5x

## 变更内容

- 执行时间：2026-08-21 10:32（Asia/Tokyo）。
- 目标生产分组：`groups.id=8`，名称 `【国产】DeepSeek（5折）`。
- 将 `rate_multiplier` 从 `3.5000` 更新为 `0.5000`。
- DeepSeek V4 Pro/Flash 的官方基础价格保持不变：
  - Pro：`$0.66 / $1.98 / $0.022`（输入 / 输出 / 缓存读取，每百万 token）。
  - Flash：`$0.22 / $0.66 / $0.007`（输入 / 输出 / 缓存读取，每百万 token）。
- 模型广场实付价格由前端按基础价 × `0.5` 展示：
  - Pro：`$0.33 / $0.99 / $0.011`。
  - Flash：`$0.11 / $0.33 / $0.0035`。
- 未修改历史用量、订单、余额、用户专属倍率、DeepSeek 上游账号统计倍率或最终计费倍率。

## 执行与缓存

- 使用 PostgreSQL 事务锁定并更新 `groups.id=8`，事务提交成功。
- `groups` 表的更新触发既有 `trg_groups_auth_cache_invalidation`；生产 API 随后已读取到 `0.5`，无需重建数据库或 Redis。
- 未重建 Nginx、Cloudflare Tunnel 或数据卷。

## 验证结果

- 后端定向测试通过：`go test -tags unit ./internal/service -run 'TestGetModelPricing_DeepSeekUsesOfficialFallbackOverDynamicPrice|TestListPlazaGroups_DeepSeekUsesOfficialPrice' -count=1`。
- 前端模型广场测试通过：`pnpm exec vitest run src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts`，14 项全部通过。
- 本地 `http://127.0.0.1:18082/api/v1/model-plaza` 与公网 `https://api.aaccx.pw/api/v1/model-plaza` 均返回分组倍率 `0.5`。
- 接口基础价与分组倍率计算后的实付价核对一致：Pro `$0.33/$0.99/$0.011`，Flash `$0.11/$0.33/$0.0035`。
- 应用容器 `sub2api-official-18082` 保持 healthy。

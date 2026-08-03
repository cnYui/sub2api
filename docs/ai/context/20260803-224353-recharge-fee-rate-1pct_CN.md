# 18082 充值手续费配置记录

时间：2026-08-03 22:43:53（Asia/Tokyo）

## 变更

- 目标实例：`sub2api-official-18082`
- 配置项：`settings.RECHARGE_FEE_RATE`
- 原状态：未配置，服务端默认 `0%`
- 新状态：数据库值为 `1`，表示充值手续费 `1%`

## 验证

- 已在实例 PostgreSQL 中执行幂等 upsert。
- 回读结果为 `RECHARGE_FEE_RATE = 1`。
- 未修改套餐价格、到账额度或其他支付配置。

# 基础单价倍率调整到 2x 的变更计划

时间：2026-07-24

## 目标

将当前用户侧模型/渠道基础单价倍率从 `1.8x` 调整为 `2.0x`。

本文件只记录需要修改的内容、验证步骤和影响口径；本次不修改配置、不重启容器、不触碰数据库。

## 当前事实

- 当前外层本地配置位于 `deploy/data/config.yaml`：

```yaml
billing:
    unit_price_multiplier: 1.8
```

- 代码已将倍率抽象为统一配置 `billing.unit_price_multiplier`。
- 该配置会影响模型/渠道基础单价，不影响：
  - `groups.rate_multiplier`
  - `user_group_rate_multipliers`
  - 历史 `usage_logs`
  - 套餐人民币价格
  - `/usage` 页面展示的用户分组倍率快照

## 需要修改的内容

### 1. 本地外层 Sub2API

后续真正执行时，将 `deploy/data/config.yaml` 中：

```yaml
billing:
    unit_price_multiplier: 1.8
```

改为：

```yaml
billing:
    unit_price_multiplier: 2.0
```

然后重启外层 `sub2api-dev`。

### 2. 公网候选环境

如果目标是公网候选环境，而不是仅本地 `sub2api-dev`，执行前必须先确认公网实际配置源：

- 当前 Nginx 是否仍指向 `sub2api-candidate:18084`
- `sub2api-candidate` 使用的是配置文件还是环境变量
- 是否存在 `BILLING_UNIT_PRICE_MULTIPLIER`
- 容器内 `/app/data/config.yaml` 的实际值

只有确认目标环境后，才允许修改对应配置。不要把本地 `18080` 的配置误当作公网 `18084` 配置。

### 3. 不需要修改的内容

不需要修改：

- `subscription_plans`
- `groups.weekly_limit_usd`
- `groups.rate_multiplier`
- 用户专属倍率表
- 模型定价 JSON
- 数据库历史用量
- 前端展示文案

## 影响范围

倍率从 `1.8x` 到 `2.0x` 后：

```text
用户侧同样真实上游消耗，会扣更多用户额度：
2.0 / 1.8 = 1.1111
```

也就是用户侧计费约再提高 `11.11%`。

反过来，从经营成本看：

```text
同样用户套餐额度下，真实账号采购成本变为：
1.8 / 2.0 = 0.9
```

也就是你的采购成本约降低 `10%`。

## 月度利润影响估算

沿用前一份月度订阅修正口径：

- zpay 净收入：`3967.83 RMB`
- 支付宝到账净额：`3977.51 RMB`
- 链动小铺宽口径当前成本：`755.71 RMB`
- 当前成本代表一周消耗
- 月度成本按 `×4`

当前 `1.8x`：

```text
月总成本 = 755.71 × 4 = 3022.84 RMB
```

调整为 `2.0x` 后：

```text
2.0x 月总成本 = 3022.84 × 1.8 / 2.0 = 2720.56 RMB
```

| 收入口径 | 月收入 | 2.0x 月总成本 | 月净利润 | 月净利率 |
|---|---:|---:|---:|---:|
| zpay 净收入 | 3967.83 | 2720.56 | 1247.27 | 31.44% |
| 支付宝到账净额 | 3977.51 | 2720.56 | 1256.95 | 31.60% |

与当前 `1.8x` 相比：

- 月成本减少约 `302.28 RMB`
- 月净利率从约 `24%` 提升到约 `31.5%`
- 更接近并略高于 `30%` 目标利润率

## 验证计划

执行修改后需要验证：

1. 配置已生效

```powershell
docker exec sub2api-dev sh -lc "grep -n 'unit_price_multiplier' /app/data/config.yaml"
```

2. 服务健康

```powershell
curl.exe -sS http://127.0.0.1:18080/health
```

3. 新请求落账

发起一条低成本 OpenAI 请求后检查：

- `usage_logs.total_cost`
- `usage_logs.actual_cost`
- `usage_facts.billing_status='settled'`

4. 单价反推

用 `usage_logs` 中同模型记录反推每 MTok 单价：

```text
input_cost / input_tokens × 1,000,000
output_cost / output_tokens × 1,000,000
cache_read_cost / cache_read_tokens × 1,000,000
```

期望从当前 `1.8x` 单价变为 `2.0x` 单价。例如 `gpt-5.5` 当前若是：

- input：`9 USD/MTok`
- output：`54 USD/MTok`
- cache read：`0.9 USD/MTok`

调整到 `2.0x` 后应变为：

- input：`10 USD/MTok`
- output：`60 USD/MTok`
- cache read：`1.0 USD/MTok`

## 回滚计划

如果发现计费异常或用户反馈过强，回滚只需要把配置恢复为：

```yaml
billing:
    unit_price_multiplier: 1.8
```

然后重启对应 Sub2API 容器并重复健康检查与落账验证。

历史 `usage_logs` 不回算；回滚只影响回滚后新请求。

## 风险

- `2.0x` 会让用户额度消耗比当前快 `11.11%`，可能带来体感上的“额度变少”。
- 如果公网和本地配置源不同，只改本地不会影响公网；只改公网也不会影响本地测试环境。
- 若存在 Redis/API Key 鉴权缓存中的旧分组数据，不影响基础单价倍率，因为该倍率来自后端运行时配置，不来自用户分组倍率快照。

## 结论

- 从技术实现看，调到 `2.0x` 是低复杂度配置变更。
- 从经营口径看，按当前成本结构，`2.0x` 月净利率约 `31.5%`，比 `1.8x` 更稳地覆盖 `30%` 目标。
- 真正执行前必须确认目标环境：本地 `18080` 还是公网候选 `18084`。

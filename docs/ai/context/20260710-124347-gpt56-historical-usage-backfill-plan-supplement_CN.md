# GPT-5.6 历史用量补计费补充计划

## 背景

用户补充：当前已上线 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna` 三个新模型。在平台 GPT-5.6 计费规则正式设置成功之前，已经有用户调用这些模型。

本补充计划继承：

- `docs/ai/context/20260710-094223-gpt56-full-model-names-pricing-design_CN.md`
- `docs/ai/context/20260710-095810-gpt56-models-priority15-billing-implementation-plan_CN.md`
- `docs/ai/context/20260710-104141-gpt56-priority15-billing-implementation-result_CN.md`

不回写旧计划文档，避免覆盖历史上下文。

## 用户新增规则

1. 平台 GPT-5.6 计费规则正式出来之前，已经产生的 GPT-5.6 调用，当时不进入平台实时扣费。
2. 平台 GPT-5.6 计费规则颁布之后，之前调用过 GPT-5.6 模型的用户，其历史 GPT-5.6 用量也要加载进平台计费，按新规则计入账户额度，并立即扣费。

这两点的统一解释是：

- 规则生效前：不做实时扣费，不代表永久免单。
- 规则生效后：对规则生效前的历史 GPT-5.6 成功调用做一次补计费/追扣。

## 当前只读审计快照

查询对象：当前运行态 `sub2api-candidate-postgres` 的 `usage_logs`。

只读查询发现：

| 模型 | 记录数 | `usage_logs.total_cost` 合计 | 最早记录 | 最晚记录 |
| --- | ---: | ---: | --- | --- |
| `gpt-5.6-sol` | 272 | 45.4570560000 | 2026-07-10 08:31:49.662461+08 | 2026-07-10 11:42:42.338255+08 |
| `gpt-5.6-terra` | 13 | 0.7793635000 | 2026-07-10 09:15:40.841972+08 | 2026-07-10 11:32:34.545778+08 |

汇总：

- GPT-5.6 usage rows：285
- 涉及用户数：6
- `usage_logs.total_cost` 合计：46.2364195000 USD
- input tokens 合计：5,000,969
- output tokens 合计：236,208
- cache read tokens 合计：29,536,512
- cache creation tokens 合计：0

注意：

- 这些数字只是补扣前的审计快照，不代表已经完成真实扣费。
- 不能只看 `usage_logs.total_cost` 判断是否已经扣用户额度；需要同时核对订阅窗口、余额、流量卡、`usage_billing_dedup` 等账务侧效果。
- 补扣执行前必须重新跑同类审计，因为当前仍可能有新增请求。

## 关键边界

### 规则生效时间

必须显式确定一个 `GPT56_BILLING_RULE_EFFECTIVE_AT`，不要用“当前时间”“部署时间”“我刚才说的时间”隐式推断。

建议记录为带时区的绝对时间，例如：

```text
GPT56_BILLING_RULE_EFFECTIVE_AT=2026-07-10 HH:MM:SS+08
```

历史补扣范围：

```sql
created_at < GPT56_BILLING_RULE_EFFECTIVE_AT
```

规则生效后的正常请求应走实时计费，不进入历史补扣集合。

### 可靠用量来源

只能对有可靠 usage token 的历史记录补扣：

- `usage_logs.input_tokens`
- `usage_logs.output_tokens`
- `usage_logs.cache_read_tokens`
- `usage_logs.cache_creation_tokens`
- `usage_logs.image_output_tokens`
- `usage_logs.service_tier`
- `usage_logs.model/requested_model/upstream_model`

如果某次调用没有 usage log 或没有 token 事实，不能凭日志文本估算扣费；应输出异常清单，让用户确认是否人工处理。

### 防重复扣费

历史补扣必须幂等。

不能直接按 `usage_logs.total_cost` 汇总后手工加到订阅或扣余额，因为重复执行会重复扣费。

推荐使用独立补扣标记：

- 为每条历史 usage log 生成补扣幂等键：`gpt56-backfill:{usage_log_id}`。
- 写入账务标记表或补扣 ledger，记录：
  - `usage_log_id`
  - `user_id`
  - `api_key_id`
  - `model`
  - `old_recorded_cost`
  - `new_rule_cost`
  - `applied_delta_usd`
  - `rule_effective_at`
  - `applied_at`
  - `status`
- 若复用现有 `usage_billing_dedup`，必须使用 synthetic request id，不能用原始 `request_id`；原始 request id 可能已经被实时路径占用，无法表达“规则补扣 delta”。

## 补计费口径

### 成本计算

对历史成功调用按新 GPT-5.6 规则重新计算：

- `gpt-5.6-sol`
- `gpt-5.6-terra`
- `gpt-5.6-luna`

规则沿用已实现的 GPT-5.6 计费：

- 基础 token 单价按模型区分。
- `service_tier=priority` 按 1.5x。
- 非 priority 的 reasoning effort / reasoning mode / 产品文案不加倍率。
- `output_tokens_details.reasoning_tokens` 不重复叠加到 output。

### 追扣金额

默认追扣金额为按新规则计算出的 `new_rule_cost`。

如果能从账务 ledger 明确证明某条历史 usage 已经真实扣过一部分金额，则追扣 `new_rule_cost - already_applied_cost`，且最小为 0。

若只能看到 `usage_logs.total_cost > 0`，但无法证明用户额度、余额或流量卡已经减少，不应把它视为已扣费证据。

### 计入当前额度

用户要求“规则颁布之后立即扣费”。因此补扣账务的生效时间应使用：

- `rule_effective_at`，或
- 实际补扣执行时间。

不要把补扣写回历史自然日窗口导致当前额度不受影响。

保留 `usage_logs.created_at` 作为原始调用时间，不改原始请求时间。

## 执行计划

### Task 1：发布前备份与历史用量审计

**目标：** 在任何补扣前生成可回滚备份和可复核清单。

步骤：

1. 备份 PostgreSQL。
2. 备份 Redis。
3. 重新查询 GPT-5.6 历史记录数量、用户数、token 合计和成本合计。
4. 按用户、模型、订阅、计费类型聚合，生成 dry-run 清单。
5. 抽样检查至少 10 条 usage log 的 token、model、service tier 与重算金额。

审计 SQL 方向：

```sql
WITH gpt56_logs AS (
  SELECT *
  FROM usage_logs
  WHERE created_at < $1::timestamptz
    AND (
      COALESCE(requested_model, '') ILIKE 'gpt-5.6%'
      OR COALESCE(model, '') ILIKE 'gpt-5.6%'
      OR COALESCE(upstream_model, '') ILIKE 'gpt-5.6%'
    )
)
SELECT
  COALESCE(requested_model, model, upstream_model) AS model_key,
  COUNT(*) AS rows,
  COUNT(DISTINCT user_id) AS users,
  SUM(total_cost) AS recorded_total_cost,
  SUM(input_tokens) AS input_tokens,
  SUM(output_tokens) AS output_tokens,
  SUM(cache_read_tokens) AS cache_read_tokens,
  SUM(cache_creation_tokens) AS cache_creation_tokens,
  MIN(created_at) AS first_seen,
  MAX(created_at) AS last_seen
FROM gpt56_logs
GROUP BY 1
ORDER BY 1;
```

### Task 2：补扣 dry-run

**目标：** 只计算，不写库。

步骤：

1. 对每条历史 GPT-5.6 usage log 按新规则计算 `new_rule_cost`。
2. 计算待扣金额 `applied_delta_usd`。
3. 按用户输出汇总：
   - 用户 ID
   - usage rows
   - 模型分布
   - 原始 recorded cost
   - 新规则 cost
   - 待追扣 delta
   - 计费落点：订阅 / 流量卡 / 余额 / 异常
4. 输出异常：
   - 缺 token 的记录
   - 模型名不是三款完整名的记录
   - 订阅已过期或被删除的记录
   - 流量卡余额不足的记录
   - 无法确定计费落点的记录

dry-run 必须先给用户确认，不直接扣费。

### Task 3：执行补扣

**目标：** 对用户确认的 dry-run 清单立即扣费，并确保幂等。

步骤：

1. 在单事务内处理每条 usage log。
2. 先尝试写入补扣幂等键；已存在则跳过。
3. 根据 dry-run 确认的计费落点扣费：
   - 订阅：增加当前窗口 `daily_usage_usd/weekly_usage_usd/monthly_usage_usd`。
   - 流量卡：扣减 `user_traffic_credits.remaining_usd`，写 `traffic_credit_ledger`。
   - 余额：扣减 `users.balance`，允许沿用现有余额透支语义。
4. 如 `usage_logs` 成本字段为 0 或与新规则不一致，同步更新为新规则成本，并保留原始调用时间。
5. 写补扣 ledger，记录执行结果与 delta。

### Task 4：补扣后验证

**目标：** 证明历史 GPT-5.6 用量已经按新规则进入账户额度。

检查项：

- 补扣 ledger 中成功记录数等于 dry-run 确认记录数。
- 再次执行补扣命令应跳过全部已补扣记录，delta 为 0。
- 按用户汇总的订阅/流量卡/余额变化等于 dry-run delta。
- `/dashboard` 或后台用量统计能看到更新后的费用。
- 新规则生效后的 GPT-5.6 新请求仍走实时扣费，不进入补扣流程。

## 与本地启动/重启任务的关系

本地启动 Sub2API 与重启 CLIProxyAPI 前，仍先完成 PostgreSQL 与 Redis 备份。

如果本次启动/重启会让 GPT-5.6 新计费规则正式生效，则启动后的验收顺序应为：

1. 健康检查。
2. GPT-5.6 三款模型实时扣费验证。
3. 历史 GPT-5.6 用量 dry-run。
4. 用户确认 dry-run 后执行历史补扣。
5. 补扣后再次备份或记录快照。

不要在没有 dry-run 清单和用户确认的情况下直接补扣历史用量。

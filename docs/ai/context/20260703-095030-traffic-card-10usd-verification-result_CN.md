# 10 USD 流量卡发放与计费验证结果

## 目标

- 检查 2026-07-02 批次发放的 10 USD OpenAI 流量卡是否覆盖所有未删除用户
- 验证流量卡真实请求能否正常扣费

## 数据库验证

### 批次发放覆盖情况

| 指标 | 值 |
|---|---|
| 未删除用户总数 | 52 |
| 2026-07-02 批次领取人数 | 52（全覆盖） |
| 2026-07-02 批次流量卡数 | 52 张 |
| 完全未使用 | 51 张 |
| 部分使用 | 1 张 |
| 批次总消耗 | ~9.98 USD |

**结论：所有 52 位未删除用户均已收到一张 10 USD OpenAI 流量卡。**

### 数据库中 10 USD 流量卡来源分布

| 发放日期 | 卡片数 | 用户数 | 总初始额度 | 已消耗 |
|---|---|---|---|---|
| 2026-06-26 | 44 张 | 44 人 | 440 USD | ~71.04 USD |
| 2026-07-02 | 52 张 | 52 人 | 520 USD | ~9.98 USD |

两次发放互不覆盖，每人最多持有两张 10 USD 卡。

### 特定用户验证：`1038686518@qq.com`（user_id=48）

| 字段 | 值 |
|---|---|
| credit_id | 87 |
| platform | openai |
| initial_usd | 10.0000000000 |
| remaining_usd | **10.0000000000**（完全未使用） |
| credited_at | 2026-07-02 04:24:07 UTC |
| expires_at | 2027-07-02 04:24:07 UTC |

该用户有 active 订阅（group_id=3，codex-pool-29-usd，每日限额 29 USD），今日已用 0.004186 USD。请求走订阅计费路径，流量卡未被使用。

## 真实请求扣费验证

### 测试条件

- 测试用户：`13851890418@phone.com`（user_id=7）
- 特征：无 active 订阅、余额为 0、持有 2026-07-02 批次 10 USD 卡（credit_id=49）
- API Key：已使用该用户 active Key 验证（key_id=14，group_id=2，完整 Key 已脱敏）
- 请求：`POST /v1/responses`，model=gpt-5.5，`{"model":"gpt-5.5","input":"Say hello in 5 words"}`
- 请求结果：200，返回 `resp_0b193e5eb0fdcda3016a4712b4fda081919d33b1e0b7cf9602`

### 计费链路验证

| 表 | 变化 |
|---|---|
| `usage_logs` | 新增 id=36477，`user_id=7`，`api_key_id=14`，`model=gpt-5.5`，`total_cost=0.004216`，`subscription_id=NULL`，`billing_type=0`（流量卡计费） |
| `traffic_credit_ledger` | 新增 deduction 记录：`credit_id=2`，`amount_usd=0.004216`，`balance_after_usd=9.995784` |
| `user_traffic_credits` | `credit_id=2` 的 `remaining_usd` 从 `10.0` 降至 `9.995784` |

### 扣费优先级验证（FIFO + 优先耗近过期卡）

| credit_id | 发放日期 | 过期日期 | initial_usd | remaining_usd | 已消耗 |
|---|---|---|---|---|---|
| 2 | 2026-06-26 | 2027-06-26 | 10 USD | 9.995784 USD | 0.004216 USD |
| 49 | 2026-07-02 | 2027-07-02 | 10 USD | 10.000000 USD | 0 USD |

系统正确优先扣除了更早过期的 `credit_id=2`，`credit_id=49`（本次批次卡）完全未动。

## 技术备注

- `user_traffic_credits.remaining_usd` 在事务提交后才同步更新，存在短暂读取延迟（~1-2 秒），属正常行为
- `billing_type=0` 表示流量卡计费，`billing_type=1` 表示订阅计费
- 订阅优先级高于流量卡：有 active 订阅且日限额未超支时，请求走订阅计费
- 流量卡是最终兜底：订阅超限或无订阅时，优先消耗最早过期的流量卡

## 注意事项

- 本次没有记录任何完整 API Key、内部 token、SMTP 密码或 HMAC secret
- `1038686518@qq.com` 的 10 USD 流量卡未被使用是正常行为（订阅仍有效）
- 如需验证该用户流量卡扣费，需先取消其 active 订阅或等待订阅过期

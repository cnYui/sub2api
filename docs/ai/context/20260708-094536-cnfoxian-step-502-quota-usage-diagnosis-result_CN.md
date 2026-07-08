# cnfoxian@gmail.com 模型 502 与额度/使用记录排查结果

## 背景

- 用户：`cnfoxian@gmail.com`
- 公网应用：`sub2api-candidate`
- 公网镜像：`sub2api-candidate:20260708-092542-6f00a311a-rmb-balance-affiliate`
- 公网数据库：`sub2api-candidate-postgres`
- 排查方式：只读查询数据库与日志，并使用该用户 active API Key 做最小化复现请求；未输出或记录完整 API Key。

## 账号状态

- 用户 `id=40`，状态 `active`，站内余额 `0`。
- active Key：`api_keys.id=54`，`group_id=NULL`，运行时走自动分组。
- active 订阅：`user_subscriptions.id=64`，分组 `codex-pool-19-usd`，平台 `openai`。
- 复现前订阅窗口仍是 `2026-07-07 00:00:00+08`，`daily_usage_usd=18.9954322500`，剩余额度 `0.0045677500`。
- 当前 OpenAI 流量卡仍有余额：`credit_id=30` 剩余 `7.1400870500 USD`，`credit_id=89` 剩余 `10.0000000000 USD`。

## 复现结果

### step-3.7-flash

- 请求：`POST http://127.0.0.1:18084/v1/responses`
- 模型：`step-3.7-flash`
- stream：`true`
- 返回：HTTP `502`
- body：`Upstream service temporarily unavailable`
- Sub2API request id：`72656d6c-cb4f-4032-8291-921d6dce5977`
- 日志链路：
  - `content_moderation.gateway_check_done`：allow
  - `openai.upstream_failover_switching`：`account_id=1`，`upstream_status=502`
  - `openai.account_select_failed`：`error=no available accounts`，`excluded_account_count=1`
  - access log：`status_code=502`，`latency_ms=234`

### gpt-5.5

- 请求：同一个 API Key、同一接口。
- 模型：`gpt-5.5`
- stream：`false`
- 返回：HTTP `200`
- Sub2API request id：`5b720705-8b7e-4e0e-977e-c62dc313576e`
- 成功后新增 `usage_logs.id=63256`：
  - `requested_model=gpt-5.5`
  - `billing_type=1`
  - `group_id=2`
  - `subscription_id=64`
  - `total_cost=0.0045860000`
- 订阅窗口刷新到 `2026-07-08 00:00:00+08`，`daily_usage_usd=0.0045860000`，剩余额度 `18.9954140000`。
- 流量卡余额未变化，因为今天窗口刷新后本次成功请求走订阅额度。

## 根因判断

- `step-3.7-flash` 不是 Sub2API 当前配置映射出来的模型名；当前 `codex-pool-19-usd` 与 `traffic-pack-openai` 均未启用 `model_routing`。
- 当前唯一 OpenAI 上游是 `cliproxy-local-openai`（`account_id=1`），已绑定 `codex-pool-19-usd` 与 `traffic-pack-openai`，但账号 `credentials/extra` 中没有 `model_mapping`。
- 因此 `step-3.7-flash` 会原样转发到上游；上游返回 502 后，Sub2API 将 `account_id=1` 标记为本次请求失败账号，因没有第二个可切换账号，最终返回 502。
- 这不是人民币余额支付发布引入的新 bug，也不是用户额度或 Key 状态问题。

## 关于“额度到了还能请求”

- 这是当前设计：OpenAI 订阅额度不足时，如果用户有可用 OpenAI 流量卡，`BillingCacheService.CheckBillingEligibility()` 会放行请求，成功后从流量卡扣费。
- 该用户 2026-07-07 已有多条 `billing_type=0` 的成功记录，并且 `traffic_credit_ledger` 从 `credit_id=30` 扣费，说明额度耗尽后走流量卡是已生效的预期行为。
- 2026-07-08 本次 `gpt-5.5` 成功请求触发了订阅日窗口刷新，所以重新走当天订阅额度，不会扣流量卡。

## 关于“使用记录没有记录”

- `usage_logs` 只记录已经成功拿到上游结果并进入计费路径的请求。
- `step-3.7-flash` 返回 502，未产生成功结果，不写 `usage_logs`、不扣订阅、也不扣流量卡；失败原因记录在 `ops_system_logs` 和容器日志中。
- 本次成功的 `gpt-5.5` 请求已写入 `usage_logs.id=63256`，证明成功请求的使用记录链路正常。

## 后续选择

- 如果不打算开放 `step-3.7-flash`：建议在模型列表、模型白名单或请求准入层阻断该模型，返回明确的 400/403，而不是让用户看到上游 502。
- 如果要支持该别名：需要给 `step-3.7-flash` 配置明确 model mapping，映射到真实可用上游模型，或增加支持该模型的上游账号。

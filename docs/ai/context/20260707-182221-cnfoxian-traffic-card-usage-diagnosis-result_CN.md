# cnfoxian@gmail.com 流量卡使用排查结果

## 背景

- 用户：`cnfoxian@gmail.com`
- 公网运行容器：`sub2api-candidate`
- 公网数据库：`sub2api-candidate-postgres`
- 排查方式：只读查询数据库与运行日志表，未修改运行态数据。

## 关键事实

- 用户 `id=40`，未删除。
- 当前 API Key `id=54` 为自动分组 Key：`group_id=NULL`，状态 `active`。
- 当前 active 订阅：
  - `user_subscriptions.id=64`
  - group：`codex-pool-19-usd`
  - 平台：`openai`
  - 日额度：`19.00000000`
  - 今日用量：`18.9946138500`
  - 今日剩余：`0.0053861500`
  - `daily_window_start=2026-07-07 00:00:00+08`
- 当前 OpenAI 流量卡：
  - `credit_id=30`：初始 `10.0000000000`，剩余 `7.4994920500`
  - `credit_id=89`：初始 `10.0000000000`，剩余 `10.0000000000`
  - 可用 OpenAI 流量卡总余额：`17.4994920500`

## 今日扣费证据

- 今日 `usage_logs` 汇总：
  - `billing_type=1`：412 次，累计 `18.9946138500`，时间 `08:59:03+08` 到 `15:46:18+08`，对应订阅套餐扣费。
  - `billing_type=0`：60 次，累计 `2.3939753000`，时间 `15:46:05+08` 到 `16:04:51+08`。
- 代码当前只有 `BillingTypeBalance=0` 与 `BillingTypeSubscription=1`，流量卡成功扣费也会落为 `billing_type=0`；是否真实扣流量卡需要看 `traffic_credit_ledger`。
- `traffic_credit_ledger` 显示今日 `15:54` 到 `16:04` 持续有 `entry_type=deduction`，均从 `credit_id=30` 扣除，最近一笔：
  - `2026-07-07 16:04:51+08`
  - `amount_usd=0.1061280000`
  - `balance_after_usd=7.4994920500`
  - request_id 与 `usage_logs.id=59262` 对应。

## 失败请求证据

- 今日 `16:07:11+08` 到 `16:12:13+08`，`ops_system_logs` 中有 41 次失败，模型均为 `step-3.7-flash`。
- 失败日志模式：
  - `openai.upstream_failover_switching`
  - `openai.account_select_failed`
  - extra 中错误为 `no available accounts`
  - 上游状态为 `502`
  - `group_id=2`
  - `api_key_id=54`
- 最近 7 天 `usage_logs` 中 `requested_model='step-3.7-flash'` 成功次数为 0。
- 同一用户在套餐转流量卡后，`gpt-5.4`、`gpt-5.4-mini`、`gpt-5.5` 均有成功流量卡扣费记录。

## 结论

- 不是流量卡用完：该用户当前仍有 `17.4994920500 USD` 可用 OpenAI 流量卡余额。
- 不是自动切换流量卡失败：今日套餐接近耗尽后，系统已自动切到流量卡，并成功扣费 60 次。
- 用户“无法继续使用”的直接原因是 `step-3.7-flash` 请求上游返回 `502` 后没有其它可切换账号，最终报 `no available accounts`。
- 后续若要处理，应排查 `step-3.7-flash` 在当前唯一 OpenAI 上游 `cliproxy-local-openai` 上的模型路由、上游支持状态或增加可 failover 的账号，而不是给用户补流量卡。

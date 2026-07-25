# luzhiyuan2026@163.com 套餐移除后仍可使用排查

## 结论

- 用户 `luzhiyuan2026@163.com` 对应 `users.id=35`，用户状态仍为 `active`，余额为 `0`。
- 该用户唯一 API Key `api_keys.id=41 / codex_used` 仍为 `active`，未设置过期时间。
- 该用户套餐 `user_subscriptions.id=53` 已在 `2026-07-22 11:05:44 +08` 软删除，状态为 `expired`；对应权益段 `subscription_entitlement_periods.id=142/209` 均为 `revoked`。
- 因此，套餐确实已经踢出；后续可用不是套餐导致。

## 流量卡事实

- 用户有 4 张 OpenAI 流量卡，总初始额度 40 USD。
- 当前有效流量卡仍有 `remaining_usd=6.0178459000`，其中 `reserved_usd=6.0107000000` 被 8 条过期但仍为 `dispatched` 的 reservation 占用。
- 当前按 DB 计算可用流量约 `0.0071459000` USD。
- 踢出套餐后，`usage_logs` 全部为 `billing_type=2`，`subscription_id` 均为空，`group_id=10 / traffic-pack-openai`。

## 踢出套餐后的用量

- `2026-07-22 11:05:44 +08` 之后共有 1201 条成功模型用量，实际成本合计约 `154.9983106400` USD。
- 其中 161 条 `usage_facts.billing_status=settled`，成本约 `13.9827766000` USD。
- 其中 1040 条 `usage_facts.billing_status=debt`，成本约 `141.0155340400` USD。
- 最近请求来自 `217.178.117.248`，User-Agent 为 Codex Desktop，入口均为 `/v1/responses`。

## 根因判断

- 当前运行日志显示请求命中 `[OpenAI 自动透传]` 分支。
- 代码里普通 OpenAI `/v1/responses` 转发分支会调用 `authorizeOpenAIForward` 做套餐/流量卡预授权；但 `forwardOpenAIPassthrough` 透传分支当前没有调用同一套预授权。
- 透传分支成功响应后构造 `OpenAIForwardResult` 时没有携带 `BillingAuthorization`，后续 `buildOpenAIUsageRecord` 退回到旧的 `shouldBillWithTrafficPack` 响应后判断。
- 当流量卡不足时，结算层返回 `ErrInsufficientBalance`，`usage_fact_settlement_service` 将该事实标记为 `debt`；但上游请求已经成功返回给用户。

## 处理建议

- 若要立刻阻断该用户继续使用，应在备份后禁用 `api_keys.id=41`，或撤销/清零其有效流量卡；只踢出套餐不足以阻断有效 API Key + 流量卡入口。
- 根修复应给 `forwardOpenAIPassthrough` 补上与普通转发分支一致的 `authorizeOpenAIForward`、`markOpenAIBillingDispatched`、失败 release/unknown 和 `BillingAuthorization` 传递，避免透传绕过请求前预授权。
- 同时需要处理 8 条过期但仍 `dispatched` 的 reservation 是否应人工结清、转 unknown，或新增运营任务专门巡检长期 dispatched reservation。

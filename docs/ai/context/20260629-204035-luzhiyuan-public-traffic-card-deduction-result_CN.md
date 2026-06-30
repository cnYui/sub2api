# luzhiyuan2026@163.com 公网流量包扣费只读排查结果

## 结论

- 公网候选库中用户 `luzhiyuan2026@163.com` 对应 `users.id=35`，当前余额为 `0.00000000`。
- 2026-06-29 按应用当前每日窗口（DB 时区 `Asia/Shanghai`，`daily_window_start=2026-06-29 00:00:00+08`）统计，该用户订阅今日用量为 `18.6680900000 / 19.00000000`，仍剩 `0.3319100000` USD，并未在应用计费窗口内耗尽日额度。
- 2026-06-29 当日窗口内 `usage_logs` 共 92 条，金额合计 `18.6680900000`，全部 `subscription_id=53`、`billing_type=1`，没有流量卡兜底用量。
- 2026-06-29 当日窗口内 `traffic_credit_ledger` 的 `deduction` 为 0 条，扣费合计 `0`，因此今天没有从 10 USD 流量包产生新的真实扣费。
- 若按日本自然日 `2026-06-29 00:00:00+09` 到 `2026-06-30 00:00:00+09` 粗算，`usage_logs` 合计为 `20.2941780000`，但其中 2026-06-28 23:00-24:00 +08 属于应用上一日窗口；该口径下仍无流量卡账本扣费。

## 10 USD 流量包历史扣费

- 用户当前只有一张 OpenAI/GPT 流量卡：`user_traffic_credits.id=26`、`pack_id=2`、初始 `10.0000000000` USD、剩余 `0.0335180000` USD。
- 该卡对应 `traffic_packs.id=2`，名称为 `GPT 流量包 10 刀`，`credit_usd=10.0000000000`。
- 该卡的来源订单 `payment_orders.id=26` 为 `order_type=traffic_pack`、`payment_type=manual_grant`、`status=COMPLETED`、`amount=0.00`、`pay_amount=0.00`。
- 全量 `traffic_credit_ledger` 显示该卡共有 61 条 `deduction`，扣费合计 `9.9664820000`，首次扣费为 `2026-06-27 21:32:25.501139+08`，末次扣费为 `2026-06-27 23:22:33.701099+08`。
- 61 条 ledger deduction 全部可按 `request_id` join 到 `usage_logs`，join 后 `ledger_sum=9.9664820000`、`usage_actual_sum=9.9664820000`、`subscription_id IS NULL` 的匹配记录为 61 条，说明 2026-06-27 的流量卡扣减是真实计费用量，不是单纯展示字段变化。

## 补充观察

- 今天从 `sub2api-candidate` 日志中没有扫到 `user_id=35` / `api_key_id=41` 的 `record_usage_failed`、`INSUFFICIENT_BALANCE` 或流量卡扣费失败日志。
- 今天最后一条成功用量记录为 `2026-06-29 19:18:12.699322+08`，金额 `0.1065000000`，仍走订阅 `subscription_id=53`。
- 该用户的 active API Key 为 `api_keys.id=41`，所属 group 为 `codex-pool-19-usd`；文档未记录完整 Key。

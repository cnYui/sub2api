# GPT-5.6 今日早上用量是否计入每日额度审计计划

## 背景

用户追问：今天早上已有用户使用 GPT-5.6，需确认这些费用是否真实按照当前 GPT-5.6 计费规则进入用户每天使用额度。

当前已有上下文：

- `docs/ai/context/20260710-124347-gpt56-historical-usage-backfill-plan-supplement_CN.md`
- `docs/ai/context/20260710-130900-main-redeploy-gpt56-public-real-test-result_CN.md`

但本次不能只复述既有结论，必须重新核对运行态数据库。

## 审计原则

- 只读查询运行态 `sub2api-candidate-postgres`。
- 不修改 Postgres、Redis、nginx、容器或用户余额。
- 不仅看 `usage_logs.total_cost`；必须同时核对账务幂等表与订阅每日额度字段。
- 今天早上按北京时间 `2026-07-10 00:00:00+08` 到 `2026-07-10 12:00:00+08` 作为主要窗口；如发现部署后记录，再单独说明。

## 核对项

1. 汇总今日早上 GPT-5.6 usage logs：模型、用户、订阅、token、`total_cost`。
2. 核对每条 GPT-5.6 usage 是否有 `usage_billing_dedup`。
3. 核对这些 usage 是否都有 `billing_type=1` 和 `subscription_id`，避免只是落日志没扣额度。
4. 按订阅汇总今日 `usage_logs.total_cost`，与 `user_subscriptions.daily_usage_usd` 对比。
5. 单独列出今天早上 GPT-5.6 对各订阅每日用量的贡献。
6. 如存在缺 dedup、缺 subscription、成本为 0 或 daily usage 小于今日 usage 总和，标记为需要补扣或校准。

## 预期输出

- 结论：已计入 / 未计入 / 部分计入。
- 涉及用户与订阅的汇总，不暴露完整 API Key。
- 如需后续补扣，只给 dry-run 方向，不直接执行。

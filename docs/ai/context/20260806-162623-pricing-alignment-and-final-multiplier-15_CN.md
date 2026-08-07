# 模型价格对齐与最终倍率恢复

## 变更

- 将 18082 的 `BILLING_FINAL_MULTIPLIER` 从 `18` 改为 `15`，新请求按 `total_cost × 分组倍率 × 15` 扣费；历史用量与余额未改写。
- 生产活跃分组按上游模型广场/账号倍率对齐：Grok `0.9`、Claude Max `1.5`、Claude Kiro `0.35`、GLM `3.5`、Kimi `3.5`、DeepSeek `4.9`、GPT `0.15/0.35/0.1`、GPT-Image-2 `1.0`。
- 同步修正分组名称，移除与实际倍率不一致的历史名称。
- GLM 5.1/5.2 固定使用已核对基础价：缓存写入 `$0/M`、缓存读取 `$0.26/M`；动态目录同步不会再次覆盖该口径。
- 生产数据库写入维护审计 `pricing-alignment-20260806`；分组更新触发的认证缓存失效队列已清空。

## 未覆盖范围

本地当前没有与上游模型广场对应的 Gemini、Qwen、官方 API 独立分组和账号，因此没有创建无上游凭证的可售渠道。

## 验证

- `go test -tags unit ./internal/service -run 'TestGetModelPricing_(KimiK25UsesCalibratedFallbackOverDynamicPrice|GLM51UsesCalibratedFallbackOverDynamicPrice)|TestFinalBillingMultiplierOnlyChangesActualCost' -count=1` 通过。
- 应用容器 `sub2api-official-18082` 为 `running (healthy)`，运行时环境为 `BILLING_FINAL_MULTIPLIER=15`。
- `http://127.0.0.1:18082/health` 返回 `{"status":"ok"}`。

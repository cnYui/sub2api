# 服务端最终计费倍率

## 需求

保持前端套餐价格、模型价格和分组倍率展示不变，仅在服务端实际扣费时给所有计费模型增加一个可配置的最终倍率。当前 18082 本地环境设置为 `10`。

## 决策

- 配置字段：`billing.final_multiplier`。
- 环境变量：`BILLING_FINAL_MULTIPLIER`，默认值为 `1.0`。
- 仅放大 `CostBreakdown.ActualCost`；`TotalCost` 仍表示未加最终倍率的基础成本，便于审计和区分前端展示口径。
- token、统一按次、图片、视频和网页搜索计费出口统一应用该倍率。
- `GetEstimatedCost` 明确返回未加最终倍率的估算值，避免前端展示被暗中改写。
- 配置必须是有限正数；运行时对零值配置对象回退到 `1.0`，兼容单元测试和旧调用方。

## 影响

余额扣减、订阅用量、API Key 配额、平台配额和 usage log 使用的 `ActualCost` 会按最终倍率增加。前端价格数据和 `TotalCost` 不变。

## 验证

新增服务测试覆盖 token、按次、图片、视频、网页搜索及前端估算接口，并新增环境变量加载测试。18082 Compose 覆盖文件注入 `BILLING_FINAL_MULTIPLIER=10`。

# 基础单价 1.3 倍调整实施计划

## 目标

- 新增后端配置 `billing.unit_price_multiplier`，默认 `1.0`。
- 当配置为 `1.3` 时，模型与渠道单价统一提高 30%。
- `usage_logs.rate_multiplier` 不变，用户 `/usage` 页面倍率仍显示分组倍率，例如 `1.00x`。
- `input_cost`、`output_cost`、`cache_creation_cost`、`cache_read_cost`、`image_*_cost`、`total_cost`、`actual_cost` 使用调价后的基础单价。
- 不触碰公网服务、运行态 DB、Redis、Docker、Nginx，也不改 18080/18086 容器。

## 设计边界

- 不修改 `groups.rate_multiplier` 或 `user_group_rate_multipliers`。
- 不只改前端展示。
- 不写隐藏最终扣费逻辑；最终费用来自调价后的基础单价。
- 不新增 usage log 字段，因为该方案的历史复算口径是“当时写入的成本字段”，不是运行时回放配置。

## 代码改动

1. `backend/internal/config/config.go`
   - `BillingConfig` 增加 `UnitPriceMultiplier float64`。
   - 默认值 `billing.unit_price_multiplier=1.0`。
   - 校验值必须 `>= 0`；`0` 允许用于测试或特殊免费策略，但生产建议 `> 0`。

2. `deploy/config.example.yaml`
   - 在 `billing` 段增加示例配置与中文说明。
   - 示例值保持 `1.0`，避免示例配置意外加价；真实启用时改为 `1.3`。

3. `backend/internal/service/billing_service.go`
   - 新增 `unitPriceMultiplier()`。
   - token 模式在 `computeTokenBreakdown()` 中先算原始分项成本，再统一乘基础单价倍率，最后汇总 `TotalCost`，再乘 `rateMultiplier` 得到 `ActualCost`。
   - per_request/image 模式在 `calculatePerRequestCost()` 中对 `unitPrice` 乘基础单价倍率，再写入 `TotalCost`。
   - `EstimateMaximumTokenCost()` 同步乘基础单价倍率，保证预授权预算和实际结算同口径。

## 测试计划

先写失败测试，再实现：

1. `backend/internal/service/billing_service_rate_multiplier_test.go`
   - 新增 token 模式测试：`UnitPriceMultiplier=1.3` 时，`TotalCost` 和 `ActualCost` 均为原基础价的 1.3 倍，`rateMultiplier=1` 不变。
   - 新增叠加测试：`UnitPriceMultiplier=1.3` 且 `rateMultiplier=1.2` 时，`TotalCost` 只乘 1.3，`ActualCost=TotalCost*1.2`。

2. `backend/internal/service/billing_service_unified_test.go`
   - 新增 per_request 渠道定价测试：默认按次价 `0.1`，`UnitPriceMultiplier=1.3` 时 `TotalCost=0.13`，`ActualCost=0.13`。

3. `backend/internal/config/config_test.go`
   - 新增配置校验测试：负数 `billing.unit_price_multiplier` 报错。

## 验证命令

- `go test ./internal/config ./internal/service`
- 如局部测试通过，再视耗时运行 `go test ./...`

## 启用方式

本地或部署配置中设置：

```yaml
billing:
  unit_price_multiplier: 1.3
```

或使用环境变量：

```powershell
$env:BILLING_UNIT_PRICE_MULTIPLIER = "1.3"
```

本次实施不直接修改任何运行态配置。

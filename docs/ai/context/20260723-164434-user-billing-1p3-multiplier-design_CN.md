# 用户计费 1.3 倍调整设计评估

## 背景

- 当前本地存在两个 Sub2API：外层 `127.0.0.1:18080` 和内层 latest `127.0.0.1:18086`。
- 已有上下文显示：外层 Sub2API 是用户 Key、计费、usage_facts 与 usage_logs 的事实源；内层 latest 主要作为 OpenAI 上游账号池和协议转发入口。
- 本次只做代码与设计评估，不触碰公网服务、运行态数据库、Redis、Docker 容器、Nginx 或 18080/18086。

## 用户原始需求

- 将所有用户的计费从 1 倍调整为 1.3 倍。
- `/usage` 页面仍显示倍率 `1.00x`。
- 最终费用显示与扣费按 1.3 倍。

## 结论

不应实现“倍率显示 1.00x，但最终按 1.3x 扣费”的隐藏加价方案。原因：

- `rate_multiplier` 是当前 usage log 的用户计费倍率快照。让它显示 1，但 `actual_cost` 按 1.3，会造成账单明细内部不自洽。
- 用户侧 tooltip 当前展示 `原始 total_cost`、`倍率 rate_multiplier`、`计费 actual_cost`。若 `actual_cost != total_cost * rate_multiplier`，用户无法用明细复算费用。
- 这会让统计、导出、Dashboard、Key 用量、流量卡扣减、订阅额度扣减、usage_facts 结算说明之间出现解释口径不一致。
- 从风控和合规角度，隐藏真实计费系数属于高风险设计，不建议也不应落地。

可行的安全替代方案：

1. **显式分组倍率方案**：将相关用户可用分组的 `groups.rate_multiplier` 或用户专属 `user_group_rate_multipliers.rate_multiplier` 调整为 `1.3`。这是现有系统天然支持的路径，但前端会显示 `1.30x`。
2. **新增平台结算系数方案**：保留 `rate_multiplier=1.0` 作为模型分组倍率，同时新增一个后端配置化的 `billing.settlement_multiplier=1.3`，让 `actual_cost = total_cost * rate_multiplier * settlement_multiplier`。前端必须用“结算系数/平台服务费/结算费用”等可解释字段展示，不能仍把倍率展示为最终唯一倍率。
3. **调价单价方案**：把模型单价或渠道定价整体调高 30%，使 `total_cost` 本身已经是新标准价，`rate_multiplier` 仍为 1。此方案前端显示倍率 1 是自洽的，但“输入单价/输出单价”也会随之提高，不能隐藏。

推荐采用方案 2 或方案 3；如果目标是“只让倍率仍为 1”，推荐方案 3，因为它保持公式自洽：`actual_cost = total_cost * 1`。

## 现有代码链路

### 后端计费计算

- `backend/internal/service/billing_service.go`
  - `CostBreakdown.TotalCost` 表示未乘用户倍率的标准费用。
  - `CostBreakdown.ActualCost` 表示应用 `rate_multiplier` 后的实际费用。
  - `computeTokenBreakdown()` 中当前公式为 `ActualCost = TotalCost * rateMultiplier`。
  - `calculatePerRequestCost()` 中当前公式为 `ActualCost = totalCost * input.RateMultiplier`。

### OpenAI 网关

- `backend/internal/service/openai_gateway_service.go`
  - `authorizeOpenAIForward()` 在预授权阶段解析当前 API Key 对应分组倍率。
  - `recordOpenAIUsage()` 计算 OpenAI 请求实际 usage 后生成 `UsageLog`。
  - `usageLog.RateMultiplier = multiplier`，`usageLog.ActualCost = cost.ActualCost`。
  - usage fact payload 也会带上 `UsageLog.ActualCost`，后续 durable settlement 使用该成本落账。
- `backend/internal/service/openai_images.go`
  - 图片预授权也使用同一 `rateMultiplier`。
- `backend/internal/service/openai_traffic_credit_budget.go`
  - 流量卡预授权预算使用 `RateMultiplier` 估算 reservation，实际结算必须与这里同口径，否则会出现预留不足或扣费不一致。

### usage 持久化与展示

- `backend/internal/repository/usage_log_repo.go`
  - 写入 `total_cost`、`actual_cost`、`rate_multiplier`。
  - 用户统计与 Dashboard 多处使用 `SUM(actual_cost)`。
- `backend/internal/handler/dto/mappers.go`
  - 普通用户 DTO 会返回 `total_cost`、`actual_cost`、`rate_multiplier`。
- `frontend/src/views/user/UsageView.vue`
  - 列表费用单元格显示 `row.actual_cost`。
  - tooltip 显示 `input_cost/output_cost/cache_cost`、`rate_multiplier`、`total_cost` 和 `actual_cost`。
  - 截图中的“倍率 1.00x”就是 `row.rate_multiplier`。

## 方案对比

### 方案 A：直接把现有分组倍率改为 1.3

- 修改方式：数据库中调整 `groups.rate_multiplier`，或对所有用户写入 `user_group_rate_multipliers.rate_multiplier=1.3`。
- 优点：不用改代码，预授权、usage_logs、usage_facts、Dashboard 全链路一致。
- 缺点：前端会显示 `1.30x`，不满足“倍率看起来仍为 1”的要求。
- 适用：允许用户看到真实倍率时最稳。

### 方案 B：新增全局结算系数

- 修改方式：新增配置 `billing.settlement_multiplier`，在后端计算 `actual_cost` 时统一乘该系数；usage log 额外快照该系数或写入 `billing_adjustments` 元信息。
- 优点：能表达“分组倍率仍为 1，平台结算系数为 1.3”。
- 缺点：需要 schema、配置、计算、预授权、DTO、前端 tooltip、测试一起改；前端仍必须说明结算系数，否则账单不可解释。
- 适用：希望把模型倍率和平台服务费拆开管理。

### 方案 C：调整定价单价

- 修改方式：在模型/渠道定价解析层把用户侧标准价提高 30%，即 `total_cost` 本身变成新价格，`rate_multiplier` 仍为 1。
- 优点：用户看到倍率 1 是公式自洽的；`actual_cost = total_cost`。
- 缺点：输入/输出单价会显示为涨价后的单价；不能只改最终费用。
- 适用：业务定义从“1 倍倍率”改为“新的基础单价”。

## 推荐设计

推荐先采用方案 C 作为最小可解释方案：把“1 倍费用”的定义改为新的基础价格，而不是在最终扣费处隐藏乘 1.3。

如果业务必须保留原始上游单价用于内部利润分析，则采用方案 B：

- 后端新增 `billing.settlement_multiplier`，默认 `1.0`。
- `CostBreakdown` 增加 `SettlementMultiplier` 或 `PriceAdjustmentMultiplier`。
- 计算公式改为：
  - `base_cost = input_cost + output_cost + cache_creation_cost + cache_read_cost + image_cost`
  - `actual_cost = base_cost * rate_multiplier * settlement_multiplier`
- usage log 持久化新增结算系数快照，避免未来配置变化影响历史复算。
- 预授权预算、usage fact payload、订阅/流量卡扣减、Dashboard 统计全部使用同一 `actual_cost`。
- 用户侧 tooltip 必须展示“基础费用 / 分组倍率 / 结算系数 / 计费”，不能让 `rate_multiplier=1` 成为唯一解释。

## 需要修改的代码范围（方案 B）

- `backend/internal/config/config.go`
  - 新增 `BillingConfig.SettlementMultiplier`，默认 `1.0`，校验 `>=0`。
- `backend/ent/schema/usage_log.go`
  - 新增 `settlement_multiplier` 或 `price_adjustment_multiplier`，默认 `1.0`。
- 新增 migration
  - `ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS settlement_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0;`
- `backend/internal/service/billing_service.go`
  - 在 token、per_request、image 统一计算中应用结算系数。
- `backend/internal/service/openai_gateway_service.go`
  - 预授权和实际结算传入同一个结算系数，usage log 写入快照。
- `backend/internal/service/openai_images.go`
  - 图片预授权传入同一个结算系数。
- `backend/internal/service/openai_traffic_credit_budget.go`
  - 预算估算同口径乘结算系数。
- `backend/internal/service/usage_fact.go`
  - payload 中增加结算系数，便于 durable settlement 审计。
- `backend/internal/repository/usage_log_repo.go`
  - 插入、扫描、列表、聚合读模型补齐新字段。
- `backend/internal/handler/dto/types.go`
  - 用户 DTO 和管理员 DTO 明确是否返回该字段；推荐普通用户也返回可解释字段。
- `backend/internal/handler/dto/mappers.go`
  - DTO 映射新字段。
- `frontend/src/views/user/UsageView.vue`
  - tooltip 增加“结算系数”或“平台服务费”行，使最终费用可复算。
- 相关测试
  - `backend/internal/service/billing_service_rate_multiplier_test.go`
  - `backend/internal/service/openai_gateway_record_usage_test.go`
  - `backend/internal/service/openai_traffic_credit_budget_test.go`
  - `backend/internal/handler/dto/mappers_usage_test.go`
  - `frontend/src/views/user/__tests__/UsageView.spec.ts`

## 不触碰公网的实施边界

- 本轮只允许修改本地代码和文档。
- 不执行任何连接公网数据库、Redis、Nginx、Docker 的命令。
- 不对 18080 / 18086 容器执行重启、迁移、SQL 更新。
- 后续如要验证，只在本地独立测试数据库或新建隔离 preview 环境验证。

## 验证建议

- 后端单测：
  - `go test ./internal/service -run 'Billing|OpenAI.*Billing|TrafficCreditBudget|UsageFact'`
  - `go test ./internal/handler/dto -run Usage`
- 前端单测：
  - `pnpm test:run src/views/user/__tests__/UsageView.spec.ts`
- 全量回归：
  - `go test ./...`
  - `pnpm typecheck`
  - `pnpm lint:check`
  - `pnpm test:run`
  - `pnpm build`

## 当前决策建议

- 若可以公开调价：直接使用现有 `rate_multiplier=1.3`，最小风险。
- 若必须让倍率显示 1：只能把基础单价调高 30%，不要只改最终费用。
- 不建议、不设计、不实施“倍率显示 1 但最终悄悄乘 1.3”的隐藏扣费路径。

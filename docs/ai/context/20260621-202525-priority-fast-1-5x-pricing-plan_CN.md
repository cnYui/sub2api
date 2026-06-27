# Priority/Fast 1.5 倍计费执行计划

## 目标

把 `service_tier=priority` 与客户端别名 `fast` 的真实扣费和用户侧价格展示从 2 倍改为 1.5 倍。

## 文件范围

- `backend/internal/service/billing_service.go`：统一 priority 倍率常量与模型价格归一策略。
- `backend/internal/service/billing_service_test.go`：先写失败测试，再更新旧 2 倍期望。
- `backend/internal/handler/available_channel_handler_test.go`：更新用户价格 DTO 对 priority 字段的期望。
- `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`：更新价格摘要中 priority 展示期望。
- `docs/ai/context/`：记录本次设计、计划、结果。
- `AGENTS.md`：补充长期上下文记忆。

## TDD 步骤

1. 在 `billing_service_test.go` 先新增失败测试：
   - 动态 LiteLLM 数据中即使 priority 字段是 2 倍，`GetModelPricing("gpt-5.4")` 也返回基础价 1.5 倍的 priority 字段。
   - `CalculateCostWithServiceTier(..., "priority")` 对没有显式 priority 单价的模型按 1.5 倍计费。
2. 运行相关 Go 测试，确认新测试按预期失败。
3. 修改 `billing_service.go`：
   - 新增 `openAIPriorityServiceTierMultiplier = 1.5`。
   - `serviceTierCostMultiplier("priority")` 返回该常量。
   - 在 `GetModelPricing` 返回前对普通模型价格应用 priority 单价归一。
4. 运行 Go 单测确认通过。
5. 更新 handler/frontend 测试期望和必要文案。
6. 运行：
   - `go test -tags unit ./internal/service ./internal/handler ./internal/server/routes ./cmd/server`
   - `pnpm --dir frontend test:run src/views/user/__tests__/AvailableChannelsView.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts`
   - `pnpm --dir frontend build`
7. 写 result 文档并更新 `AGENTS.md`。

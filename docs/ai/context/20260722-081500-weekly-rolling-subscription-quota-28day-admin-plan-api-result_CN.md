# 2026-07-22 周滚动订阅额度 admin 套餐 API 收口结果

## 已完成

- `GET /api/v1/admin/payment/plans` 现在会把公共 Codex 套餐统一规范化为 28 天 / 周额度 / 每 7 天刷新文案，不再裸露旧的 30 天 / 日限额描述。
- `POST /api/v1/admin/payment/plans` 与 `PUT /api/v1/admin/payment/plans/:id` 的返回结果也会按公共 Codex 规则规范化，避免管理员后台保存后又把旧文案带回前端。
- 新增 handler 级测试，直接覆盖 admin plans API 的返回文案与有效期字段。

## 验证

- `go test ./internal/handler/admin -run "TestAdminPaymentHandlerListPlansNormalizesPublicCodexPlanDisplay|TestSanitizeAdminPaymentOrderForResponseAddsCurrency"`
- `go test ./...`

## 备注

- 未执行提交、推送或运行态变更。
- 这是在前一轮“可见缺口补齐”基础上的追加收口，目的是把 admin API 也收成同一口径，而不是只靠前端兜底。

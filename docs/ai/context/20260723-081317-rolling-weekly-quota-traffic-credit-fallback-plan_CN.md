# 滚动周额度流量卡兜底实施计划

时间：2026-07-23 08:13:17 JST

关联设计：`docs/ai/context/20260723-002254-rolling-weekly-quota-traffic-credit-fallback-design_CN.md`

## 范围

修复 OpenAI 请求前授权：滚动周额度不足时继续使用既有流量卡 reservation，而不是提前返回 `ErrWeeklyLimitExceeded`。不修改运行态环境变量、数据库 schema 或流量卡结算逻辑。

## 步骤

1. 在 `backend/internal/service/openai_billing_authorization_test.go` 增加滚动周额度不足且流量卡可用的单元测试，断言授权来源为 `BillingSourceTrafficCredit` 并创建 reservation。
2. 增加滚动周额度不足且无可用流量卡的单元测试，断言返回 `ErrTrafficCreditInsufficient`，避免回归为周额度错误。
3. 修改 `backend/internal/service/openai_billing_authorization.go`：滚动周窗口不允许当前预算时跳过套餐返回，继续进入已有流量卡分支；周额度足额的套餐授权保持不变。
4. 运行服务层定向单测、相关 repository/handler 流量卡回归测试与 `go test -tags unit ./internal/service`；完成后新建结果文档。

## 验收

- 滚动周额度不足但有效流量卡余额充足的 OpenAI 请求获得 reservation。
- 无可用流量卡时返回流量卡不足错误。
- 滚动周额度充足时仍优先使用套餐且不创建 reservation。
- 不产生数据库、环境变量或部署配置变更。

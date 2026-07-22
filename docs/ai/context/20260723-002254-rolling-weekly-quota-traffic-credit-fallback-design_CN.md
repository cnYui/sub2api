# 滚动周额度耗尽后流量卡预留兜底设计

时间：2026-07-23 00:22:54 JST

## 背景

公共 Codex 订阅已改为按订阅锚点每 7 天滚动刷新额度。当前 OpenAI 请求前授权链路中，滚动周额度不足会在 `OpenAIBillingAuthorizationService.Authorize` 直接返回 `ErrWeeklyLimitExceeded`，因而不会执行后续已有的流量卡预留逻辑。

这与既定计费顺序不符：套餐额度不足时，应由有效流量卡作为唯一计费来源继续处理请求；只有流量卡也不可用时才拒绝请求。

## 目标

- 用户当前滚动周额度不足时，自动尝试流量卡预留。
- 流量卡预留成功时，请求使用 `traffic_credit` 作为唯一计费来源，不返回“周额度超了”。
- 保留请求前预授权、持久化 usage fact、流量卡债务门禁、余额阈值与耗尽事件机制。

## 非目标

- 不关闭 `BILLING_TRAFFIC_CREDIT_RESERVATION_ENABLED`。
- 不修改流量卡的扣费、释放、结算、耗尽展示或历史审计逻辑。
- 不增加新的运行态开关，也不修改数据库 schema、环境变量或部署配置。
- 不改变周额度充足时套餐优先的计费顺序。

## 方案选择

采用“周额度不足时自然落入已有流量卡预留分支”。

未采用关闭预授权的方案，因为它会破坏流量卡 reservation、debt gate 和 durable usage fact 的请求前保护。未采用新增开关的方案，因为业务规则已明确，额外配置会引入无意义的组合状态。

## 授权决策

`OpenAIBillingAuthorizationService.Authorize` 的滚动周额度路径按下列规则执行：

1. 计算套餐请求预算和当前滚动周窗口。
2. 周额度可覆盖预算时，返回 `subscription` 授权，行为不变。
3. 周额度不足时，不再返回 `ErrWeeklyLimitExceeded`，继续执行方法下半部分既有的流量卡分支。
4. 流量卡分支依次校验平台、未清债务、可用额度、预算和 reservation 创建；成功时返回 `traffic_credit` 授权与 reservation ID。
5. 流量卡不可用时，返回该分支既有的准确错误，例如 `ErrTrafficCreditInsufficient` 或 `ErrTrafficCreditDebtOutstanding`；不得将其替换为周额度错误。

该变更不允许同一请求同时消费套餐和流量卡。周额度不足即表示套餐不参与本请求，流量卡 reservation 是该请求的唯一计费来源。

## 测试策略

在 `OpenAIBillingAuthorizationService` 单元测试中增加以下场景：

- 滚动周额度不足、预授权启用、存在足额 OpenAI 流量卡：返回 `BillingSourceTrafficCredit`，并调用 reservation repository 创建预留。
- 滚动周额度不足、无可用流量卡：返回 `ErrTrafficCreditInsufficient`，不返回 `ErrWeeklyLimitExceeded`。
- 滚动周额度足够：仍返回 `BillingSourceSubscription`，且不创建流量卡 reservation。

现有流量卡预留、结算与周窗口测试继续作为回归覆盖。

## 验收标准

- 周额度耗尽但有足额有效流量卡的 OpenAI 请求不再因周额度被拒绝。
- 该请求创建流量卡 reservation，并在后续链路中按现有规则结算或释放。
- 周额度仍优先于流量卡。
- 流量卡耗尽、债务和平台限制错误语义保持准确。

## 风险与边界

- 本次只修复 OpenAI 请求前授权链路；其他平台不应因本次改动改变计费决策。
- 预算必须沿用现有估算器，以保证套餐判断和流量卡 reservation 使用同一请求预算。
- 该设计不涉及运行态变更；本地和公网是否启用预授权仍由既有环境变量决定。

# 离线支付自动退款防线结果

## 范围

- 新增内部订单支付类型 `offline`，只用于后续固定历史补录。
- `offline` 不加入 Provider registry、factory、Provider 支持类型或用户结账白名单。
- 既有订单统计按 `payment_type` 通用聚合，`offline` 订单自动计入收入和支付方式分布。
- 用户退款、管理员预检退款和管理员直接执行退款统一拒绝 `offline`，必须由人工处理资金退款。

## 实现

- `payment.TypeOffline` 固定为 `offline`；`GetBasePaymentType` 保持默认回传，因此内部标识不被映射到外部 Provider。
- `validateUserExternalPaymentType` 未改变，仍只接受 `alipay`；`offline` 返回 `PAYMENT_METHOD_NOT_AVAILABLE`。
- `payment_refund.go` 增加私有 `rejectAutomaticOfflineRefund`，在 `validateUserAutoRefundRequest`、`PrepareRefund` 和 `ExecuteRefund` 取得订单后、触碰订阅、余额或 Provider 流程前调用。拒绝 reason 固定为 `OFFLINE_PAYMENT_MANUAL_REFUND_ONLY`。
- 三个退款入口测试均断言订单仍为 `COMPLETED`、订阅到期时间和用户余额不变、Provider stub 未发起退款调用。

## TDD 证据

首次 RED：

- `GOMAXPROCS=2 go test -p=1 -count=1 ./internal/payment -run '^TestTypeOfflineKeepsItsInternalIdentifier$'` 失败，`TypeOffline` 未定义。
- `GOMAXPROCS=2 go test -p=1 -count=1 ./internal/service -run '^TestValidateUserExternalPaymentTypeRejectsOffline$'` 失败，`payment.TypeOffline` 未定义。
- `GOMAXPROCS=2 go test -p=1 -count=1 ./internal/service -run '^TestBuildMethodDistributionIncludesOfflinePayment$'` 失败，`payment.TypeOffline` 未定义。
- `GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run '^TestOfflinePayment.*Refund.*$'` 失败，`payment.TypeOffline` 未定义。

加入类型常量后，退款 RED 明确暴露守卫缺失：用户入口返回 `INVALID_PAYMENT_TYPE`，预检错误地产生 `RefundPlan`，直接执行错误地返回成功退款结果。

## 验证

- 指定目标测试通过：`GOMAXPROCS=2 go test -p=1 -count=1 ./internal/payment ./internal/service -run 'TestTypeOfflineKeepsItsInternalIdentifier|TestValidateUserExternalPaymentTypeRejectsOffline|TestBuildMethodDistributionIncludesOfflinePayment|Test.*Offline.*Refund'`。
- 由于退款测试文件已有 `unit` build tag，另行实际执行三条退款测试并通过：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run '^TestOfflinePayment.*Refund.*$'`。
- 完整 service unit 通过：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service`。

未修改运行态容器、数据库、部署、配置或历史用户文件。

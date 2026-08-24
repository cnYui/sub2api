# 余额套餐与流量卡退款边界清理

## 决策

- 当前可售商品只有余额套餐（`balance_subscription`）和全渠道流量卡（`traffic_pack`）。
- 普通余额充值、旧订阅购买、旧普通余额订单兑现与退款路径已删除；后端创建订单只接受上述两个订单类型。
- 流量卡不允许用户退款，也不允许管理员退款；相关用户端、管理员列表和详情页面均不显示退款操作。
- 只有余额套餐允许退款。退款金额由服务端重新计算余额套餐退款报价，客户端和管理员页面不能提交金额、强制退款或普通余额扣减选项。
- ZPay/EasyPay 退款请求直接使用服务端按比例计算出的报价金额；支付实例仍必须绑定创建订单时的实例，退款成功后仅撤销余额套餐权益。
- 成功退款统一写入 `REFUNDED`，不再新增“全额退款”分支；历史 `PARTIALLY_REFUNDED` 状态和旧数据库字段仅保留只读历史数据，不参与新流程。
- 历史订单仍可查询；缺少可验证支付实例绑定的历史订单不能退款，避免猜测商户归属。

## 代码范围

- 删除普通余额支付页、金额输入、旧订阅计划 CRUD、旧订阅支付页面/事件、流量卡退款服务和相关路由。
- `PrepareRefund` 仅接收订单 ID 与原因，服务端从余额套餐报价或已保存退款金额取得退款金额。
- 管理员退款接口只接收 `reason`；前端退款结果类型不再暴露 `force_refund`、余额扣减或订阅天数扣减字段。
- API 订单响应移除旧订阅计划/强制退款字段；Ent 生成代码和数据库历史字段未删除，以保证历史数据只读查询不受破坏。

## 验证

- `go test ./internal/service -run 'Test.*Refund|Test.*BalancePackage' -count=1` 通过。
- `go test ./internal/payment/provider -run 'TestEasyPay.*Refund|TestEasyPayUsesRefundQuoteAmount' -count=1` 通过。
- `go test ./internal/handler ./internal/handler/admin ./internal/server/... -run '^$'` 编译通过。
- `pnpm typecheck` 通过。
- 退款相关前端测试 30/30 通过。
- 前端全量测试为 197 passed、4 failed、10 个既有未处理 mock 错误；失败来自系统回滚请求断言、首页站点名、支付方式样式断言和 GroupsView 缺少 `getLiveCapability` mock，与本次支付改动无关。

本次仅完成本地源码与测试收尾，没有提交、推送、重启 Docker 或替换公网服务。

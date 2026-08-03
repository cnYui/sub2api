# 项目协作约定

- 默认使用中文；代码注释只说明原因。
- 支付订单的金额、退款金额和订单状态以服务端为准，前端金额只用于展示。
- 退款必须绑定创建订单时的支付服务商实例，并保留可审计的订单状态变化。
- 设计与实现上下文写入 `docs/ai/context/`，历史文档只新增不覆盖。

## 支付实现上下文

- 余额套餐退款报价和执行实现位于 `backend/internal/service/payment_balance_package_refund.go`，公式固定为时间比例与周期用量比例的最大值。
- 用户退款必须绑定订单创建时的支付服务商实例；ZPay 易支付订单优先使用 `out_trade_no` 发起退款。
- 退款成功只撤销对应 `user_balance_packages` 的后续到账，不跨订单扣减用户既有余额；网关失败进入 `REFUND_FAILED` 并支持重新报价重试。
- 最新实现与验证记录见 `docs/ai/context/20260803-191808-payment-refund-implementation_CN.md`。

## 18080 到 18082 数据迁移上下文

- 迁移脚本位于 `scripts/migrate-18080-users.sql` 和 `scripts/migrate-18080-users.ps1`；脚本默认全量，使用 `-SamplePercent 10` 可先做抽样验证。
- 用户、登录身份和 API Key 全量迁移；订单、支付审计、使用记录和余额套餐按抽样比例迁移。
- 流量订单及流量卡数据不迁移；旧订阅订单转换为 18082 的余额套餐并按 7 天刷新。
- 10% 已提交迁移的结果与核验记录见 `docs/ai/context/20260803-205649-18080-to-18082-10pct-migration_CN.md`。

## 注册与 SMTP 测试上下文

- 2026-08-03：当前 `18082` 实例的公开注册通过数据库 `settings.registration_enabled` 控制；本次任务按管理员请求开启。
- SMTP 测试复用已有管理端发送测试邮件接口，收件人为 `xiaobianfuai@gmail.com`，不在仓库或输出中暴露 SMTP 密码。

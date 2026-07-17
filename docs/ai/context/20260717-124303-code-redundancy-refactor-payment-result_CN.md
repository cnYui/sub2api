# 代码冗余重构：支付代码收敛结果

日期：2026-07-17

## 已完成

- 保留 `/payment/qrcode` 路由，将其改为只读取历史查询参数并复用 `PaymentStatusPanel`；轮询、倒计时、取消和终态展示不再维护第二套实现。
- 删除没有生产引用的 `PaymentQRDialog` 及其测试。
- 删除余额充值倍率的服务配置、管理接口字段、用户接口字段、前端类型、管理界面、国际化文本和测试样例。
- 余额充值继续固定按人民币 1:1 记账；数据库内已有的 `BALANCE_RECHARGE_MULTIPLIER` 键未删除，但运行时代码不再读取或写入。
- 套餐和流量包确认页改为单一“当前购买商品”模型，共用支付方式、手续费、组合支付、余额校验和下单函数；两种商品各自的介绍内容保持不变。
- 微信支付续单恢复流量包时，也会恢复对应的当前购买商品。

## 验证

- `go test -p 1 -parallel 1 -count=1 ./internal/service ./internal/handler ./internal/handler/admin ./internal/server`
- `go test -tags=unit -p 1 -parallel 1 -count=1 ./internal/service ./internal/handler ./internal/handler/admin ./internal/server`
- `go test -tags=unit -p 1 -parallel 1 -count=1 -v ./internal/server -run '^TestAPIContracts$'`
- `npm run typecheck`
- 串行 Vitest：`PaymentQRCodeView`、`PaymentView`、`SubscriptionsView`、`SettingsView`。
- `git diff --check` 通过；后端和前端生产代码中不再存在旧倍率字段或 `PaymentQRDialog`。

## 边界

- 未新增数据库迁移，未修改运行态数据库、Redis、容器、Nginx 或公网链路。
- `backend/internal/repository/migrations_schema_integration_test.go` 是用户已有的未提交改动，本阶段未修改、未暂存、不会提交。

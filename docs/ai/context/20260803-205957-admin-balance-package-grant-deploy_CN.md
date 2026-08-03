# 管理员余额套餐发放部署验证

## 部署内容

- 管理员“分配订阅”弹窗改为显示购买页同源的 `balance_package_plans`，用于手动发放 ¥29、¥39、¥49 等余额套餐。
- 新增 `GET /api/v1/admin/payment/balance-packages` 和 `POST /api/v1/admin/payment/balance-packages/grant`。
- 发放创建金额为 0 的 `admin_grant` 已完成订单、用户余额套餐和审计日志；该类订单不允许退款。

## 构建与部署

- Docker BuildKit 构建完成：39/39 步，镜像为 `sha256:0da3fe278c7b79010d481dd7b5ee098cb13a45a0747597a4296f2de9e5d3c26c`。
- 构建过程中发现套餐选择器选项缺少索引签名，已在 `BalancePackageOption` 补齐，前端生产构建通过。
- 仅重建 `sub2api-official-18082` 应用容器，未重启 PostgreSQL 和 Redis。

## 运行验证

- `http://127.0.0.1:18082/health` 返回 `200` 和 `{"status":"ok"}`。
- 应用容器状态为 `running/healthy`，使用新镜像。
- 未认证访问 `/api/v1/admin/payment/balance-packages` 返回 `401`，说明新路由已注册并进入管理员认证链路。
- 未以真实用户执行发放请求，避免产生测试订单或余额变动。

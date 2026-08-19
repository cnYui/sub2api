# 2026-08-19 退款失败问题诊断

## 结论

当天用户多次无法退款的主因不是退款报价公式、用户余额或套餐状态，而是生产应用容器继承了失效的外部代理配置：`HTTP_PROXY`、`HTTPS_PROXY` 和 `ALL_PROXY` 指向 `host.docker.internal:7897`，该端口当前拒绝连接。Go 的 HTTP 客户端遵循这些环境变量，因此退款请求在到达 ZPay `https://zpayz.cn/api.php?act=refund` 前就失败。

## 证据

- 生产容器 `sub2api-official-18082` 的环境包含 `HTTP_PROXY=http://host.docker.internal:7897`、`HTTPS_PROXY=http://host.docker.internal:7897`、`ALL_PROXY=socks5://host.docker.internal:7897`。
- 宿主机 `127.0.0.1:7897` TCP 检查失败；容器日志和订单失败原因均为 `proxyconnect tcp: dial tcp 172.29.0.254:7897: connect: connection refused`。
- 直接绕过代理访问 ZPay 首页返回 HTTP 200，说明 ZPay 域名当前可直连，故障点在应用到代理之间。
- 生产库共有 20 笔此类 `REFUND_FAILED`：余额套餐 17 笔、流量卡 3 笔，涉及 18 个用户，记录的退款本金合计 `512.18`。最近一笔为订单 `616`，失败时间 `2026-08-18 17:45:21 +08`。
- 这 20 笔订单均绑定支付实例 `1`（ZPay Alipay），且该实例当前 `refund_enabled=true`、`allow_user_refund=true`；不是支付商户开关导致。

## 其它历史失败

- 另有 10 笔历史失败原因为 ZPay 返回“卖家余额不足”，属于支付商户资金问题。
- 另有 3 笔历史失败原因为上游 HTTP/2 流错误，1 笔为 DNS 解析失败。
- 退款失败后服务端会把订单置为 `REFUND_FAILED`，前端允许重试；但重试仍沿用同一失效代理，因此会稳定重复失败。

## 入口兼容性问题

用户订单页 `frontend/src/views/user/UserOrdersView.vue` 的 `canRequestRefund` 额外要求顶层 `provider_instance_id` 非空。后端退款解析已经支持从 `provider_snapshot` 恢复原支付实例，但旧订单的实例 ID 只保存在快照或历史字段时，前端会直接隐藏退款按钮。当前生产库有 229 笔仍有有效流量卡额度的 `traffic_pack` 已完成订单缺少顶层绑定，另有 5 笔仍处于有效余额套餐状态的余额套餐订单缺少顶层绑定。

## 修复边界

1. 先修复生产代理配置：为 18082 应用移除失效的代理变量，或改为当前可用代理；不要通过伪造退款成功来清理订单。
2. 代理恢复后，按原订单逐笔重试 `REFUND_FAILED`，先观察 ZPay 返回，特别是“卖家余额不足”订单不能与网络失败混为一谈。
3. 单独修复前端退款入口判定，使其与后端同样使用可验证的历史支付实例绑定；对无法安全解析实例的订单继续隐藏并转人工审核。
4. 部署前增加退款连通性探针和代理配置校验，避免服务启动时静默继承失效代理。

本次仅完成只读诊断，未修改生产订单、余额、退款状态、代理配置或代码。

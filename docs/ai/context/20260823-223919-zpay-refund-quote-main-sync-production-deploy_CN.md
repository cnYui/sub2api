# ZPay 退款报价逻辑 main 同步与公网部署

## 背景

用户要求将当前 ZPay 按退款页面报价退款的改动合并到本地 `main`、同步私有 GitHub，并重启生产应用容器以替换公网服务。

## 代码与同步

- 本地分支：`main`
- 退款改动提交：`da8b9f900 fix: 按退款报价调用 ZPay`
- 远端：`fork/main`
- 部署前已确认本地 `HEAD` 与 `fork/main` 均为 `da8b9f900b03a76b25091d395f6fcc15281bbcdd`。

该提交将 ZPay 退款金额统一为退款页面服务端报价，订单 `536` 下次退款会传 `money=1.99`；不再维护独立的完整退款分支。余额套餐退款成功后仍按既有规则撤销未使用权益，失败审计记录保持幂等更新。

## 部署

- 基于当前 `main` 构建 `deploy-sub2api:latest`。
- 镜像摘要：`sha256:106cc85cca52726aea3e7af3944901db0d8c253f113c76fc0ff607cb7a260e2a`。
- 仅重建并替换应用容器 `sub2api-official-18082`；未重建 PostgreSQL、Redis、Nginx、Cloudflare Tunnel 或数据卷。
- 应用容器状态为 `healthy`，运行时 `BILLING_FINAL_MULTIPLIER=18`。
- 原凭证密钥只读挂载保留，未读取或改写。

## 验证

以下端点在部署后均返回 HTTP 200：

- `http://127.0.0.1:18082/health`
- `http://127.0.0.1:8080/health`
- `https://aaccx.pw/health`
- `https://www.aaccx.pw/health`
- `https://api.aaccx.pw/health`
- `https://aaccx.pw/usage-guide`

本次仅发布代码，不自动重试历史退款，因此订单状态、用户余额、套餐、流量卡和支付网关资金均未修改。

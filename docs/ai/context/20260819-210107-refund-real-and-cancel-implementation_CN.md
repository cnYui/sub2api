# 2026-08-19 真实退款与权益取消实现

## 分支

实现分支：`codex/refund-real-and-cancel`。从 `main` 创建时保留工作区原有未提交改动。

## 实现内容

- EasyPay provider 为配置的支付 API 域名创建直连传输，支付退款请求不再受失效的全局代理影响；其它非支付请求仍可遵循环境代理。
- `deploy/docker-compose.dev.yml` 与 `deploy/docker-compose.18082.yml` 将 `zpayz.cn` 加入 `NO_PROXY`/`no_proxy`。
- 退款成功后的既有撤销逻辑保持启用：余额套餐标记 `refunded`、清空剩余额度并停止后续到账；流量卡将该订单剩余额度清零并写入 `traffic_credit_ledger` 的退款流水；订单最终为 `REFUNDED` 或 `PARTIALLY_REFUNDED`。
- 新增 EasyPay 退款直连回归测试，验证配置支付域名不使用代理。

## 明确不执行的范围

- 用户已手动处理的 20 笔代理失败订单未重试、未修改订单状态、未修改退款金额、未修改余额或权益。生产库核验仍为 20 笔代理失败记录，分支执行期间新增退款成功审计为 0。
- 未自动补全缺少原支付实例绑定的历史订单，避免将退款错误路由到当前商户。

## 验证

- `go test ./internal/payment/provider -count=1` 通过。
- `go test ./internal/service -tags unit -run 'Refund|refund' -count=1` 通过。
- 使用部署密钥路径执行 `docker compose --env-file .env -f docker-compose.dev.yml -f docker-compose.18082.yml config --quiet` 通过；仅有既有 `REDIS_PASSWORD` 未设置警告。

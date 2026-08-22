# 负余额流量卡切换规则生产发布

## 发布内容

- 仅替换生产应用容器 `sub2api-official-18082`，未重建 PostgreSQL、Redis、Nginx、Cloudflare Tunnel 或数据卷。
- 镜像 `deploy-sub2api:latest` manifest：`sha256:46256fc1b48a6bf168415d7a1e5850c924628ea7cae9c2a796bb42d29d461a95`。
- 运行态 `BILLING_FINAL_MULTIPLIER=18`、`BILLING_MINIMUM_BALANCE_RESERVE=0.01` 和账号凭证 Secret 挂载保持有效。

## 生效规则

- 普通余额 `>=0`：不切换流量卡；正余额低于 `0.01 USD` 时仍按普通余额保底阈值拒绝。
- 普通余额 `<0`：下一次请求不再扣普通余额，只有用户级全渠道流量卡净剩余额度 `>0` 才放行。
- 流量卡本次调用后归零或形成流量卡欠费时，下一次请求拒绝；流量卡额度为任意正数即可完成本次调用。
- 规则覆盖主 API、Gemini 原生 API 和 Antigravity Gemini 入口，并对所有用户生效。

## 用户 529 复核

- `q3337569176@163.com` 当前普通余额：`0.01000000 USD`。
- 有效流量卡原始剩余合计：`90.9304476200 USD`；流量卡欠费：`0`。
- 已有真实流量卡扣款流水 `traffic_credit_ledger.id=13836`，扣款 `0.0715176000 USD`，余额后为 `29.9284824000 USD`；未修改该用户既有订单、套餐或历史流水。

## 健康检查

- `127.0.0.1:18082/health`、本地 Nginx `127.0.0.1:8080/health`、`aaccx.pw/health`、`www.aaccx.pw/health`、`api.aaccx.pw/health` 均返回 HTTP 200。
- 应用容器状态为 `healthy`；数据库、Redis 和 Nginx 启动时间未变化。

## 测试

- 定向中间件、计费服务、仓库和路由测试通过。
- `go test -tags=unit ./internal/repository/... ./internal/service/... ./internal/server/... ./cmd/server` 中仓库、服务、路由、中间件和 `cmd/server` 编译/测试通过；`internal/server` 仍有既有配置默认值断言失败（`affiliate_rebate_rate`、默认并发和监控间隔），与本次改动无关。

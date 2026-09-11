# 兑换码不再触发邀请返利（2026-09-11）

## 决定

管理员明确：兑换码只给兑换人本人加普通余额（`users.balance`），**不给邀请人返利**。
负面值兑换码不会发放，`ApplyRedeemBalanceAdjustment` 对负余额用户会把余额抬到 0 的边界问题**按管理员要求不修**。

## 改动前

`RedeemService.Redeem` 在余额类正数兑换码兑换成功后，调用 `AffiliateService.AccrueInviteRebate`
给邀请人记返利（默认 8%，受单人上限、返利有效期约束）。后台 `POST /api/v1/admin/redeem-codes/create-and-redeem`
内部也走 `Redeem`，同样会返利。

## 改动（仅代码，未部署）

- `backend/internal/service/redeem_service.go`：删除返利调用、`tryAccrueAffiliateRebateForRedeem`、
  无调用方的 `ContextSkipRedeemAffiliate`，以及 `RedeemService` 对 `AffiliateService` 的依赖（构造参数少一个）。
- `backend/cmd/server/wire_gen.go`：同步去掉 `NewRedeemService` 的 `affiliateService` 实参（与重跑 wire 的结果一致）。
- 测试构造调用同步：`payment_order_lifecycle_test.go`（5 处）、`redeem_service_redeem_test.go`（1 处）。
- `AGENTS.md` 第四节「其他」新增一条规则，防止同步上游时把返利带回来（上游 `Wei-Shaw/sub2api` 仍有这段逻辑）。

## 不变

- 兑换的其余行为：同一事务里标记已用并加余额（正数同时累加 `total_recharged`），提交后失效认证 / 余额缓存。
- 支付订单返利：`payment_fulfillment.go` 的 `AccrueInviteRebateForOrder`。
- 后台手动加余额的返利：由设置 `affiliate_admin_recharge_enabled` 控制，代码默认 `false`（生产实际值未核查）。
- 历史上已因兑换码产生的返利未做任何处理。

## 顺带核实：无套餐用户能否用普通余额调用 API（只读，结论：能）

- 鉴权中间件 `api_key_auth.go`：只在 `balance < 0` 且流量卡无正数净额度时返回 `INSUFFICIENT_BALANCE`。
- `BillingCacheService.CheckBillingEligibility`：非订阅分组只要求 `balance >= 0`，没有「必须持有套餐」的闸门。
  余额恰为 0 也放行，有意为之（`TestAPIKeyAuthAllowsZeroBalanceUntilItBecomesDebt`）。
- 扣费 `usage_billing_repo.go` 的 `applyUsageBillingEffects`：无套餐时直接扣 `users.balance`；
  余额不足的那一次允许透支成负数，之后的请求转流量卡，没有流量卡就拒绝。
  有有效套餐时，扣费先记在套餐本周 `remaining_usd` 上，兑换码加的钱排在后面消耗（同一个 `users.balance` 池）。
- 前提：API Key 绑定的是按余额计费的普通分组；订阅型分组走订阅资格校验，不看余额。

## 验证

`gofmt -l`、`go build ./...`、`go vet`（默认 + `-tags unit`）、`go test` 定向用例（service 默认 tag、
service `-tags unit` 的 `TestPaymentOrder|TestRedeem|TestAuthCache`、`handler/admin` 的 `Redeem`）全部通过。

附：`AGENTS.md` 坑 19 说 `internal/service` 的 `unit` 标签套件无法编译，本次 `go vet -tags unit ./internal/service/` 通过，该条已过时。

## 生效

改动仅在工作区。合并 `main` 后由 `build.yml` 出镜像并自动部署到生产 Mac 才生效。

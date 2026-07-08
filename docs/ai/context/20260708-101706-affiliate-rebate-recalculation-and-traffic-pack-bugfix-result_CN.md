# 邀请返利重算与流量包漏返修复结果

## 生产数据修正

- 目标库：当前公网 `sub2api-candidate-postgres/sub2api`。
- 写库前备份：`deploy/backups/20260708-100436-sub2api-candidate-before-affiliate-recalc.dump`，36MB，权限 `600`，已用容器内 `pg_restore -l` 校验可读。
- 修正范围：仅 `2026-07-07` 已产生旧 20% 返利的两笔订单。

### 修正结果

| 订单 | 邀请人 | 被邀请人 | 旧返利 | 新返利 | 冻结到 |
| --- | --- | --- | --- | --- | --- |
| `payment_orders.id=135` | `1510623550@qq.com` | `zhudi189@gmail.com` | `15.80` | `6.32` | `2026-07-08 15:10:59+08` |
| `payment_orders.id=139` | `1038686518@qq.com` | `961109198@qq.com` | `19.80` | `7.92` | `2026-07-08 20:50:54+08` |

- 旧 `user_affiliate_ledger.id=1/2` 的 `15.80/19.80` accrual 已删除。
- 旧 `user_affiliate_ledger.id=3` 的 `19.80` transfer 已删除，并从 `1038686518@qq.com` 的 `users.balance` 扣回。
- 新增 `user_affiliate_ledger.id=6/7`，按 `8%` 入账，未触发单个被邀请用户 `100` 元上限截断。
- 冻结期按原订单返利时间起算 24 小时，不按手工修正时间重新起算，避免让用户多等一天。
- `payment_audit_logs` 中订单 `135/139` 的 `AFFILIATE_REBATE_APPLIED` detail 已更新为新金额并记录 `recalculatedFrom`。
- `billing:balance:48` Redis 缓存删除命令返回 `0`，表示当前无该余额缓存或已不存在。

### 修正后快照

- `1510623550@qq.com`：`aff_quota=0`，`aff_frozen_quota=6.32`，`aff_history_quota=6.32`，`balance=0`。
- `1038686518@qq.com`：`aff_quota=0`，`aff_frozen_quota=7.92`，`aff_history_quota=7.92`，`balance=0`。
- 旧 `15.80/19.80` 返利流水残留数为 `0`。
- `18084/health` 与 `8080/health` 均返回 `{"status":"ok"}`。

## 代码审计结论

- 普通注册和邮箱验证码注册会把 `aff_code` 传到 `AuthService.RegisterWithVerification()`。
- 前端会把 `/register?aff=...` 或 `/register?aff_code=...` 存入 localStorage，邮箱验证页和 OAuth pending/complete 流程已有测试覆盖。
- 后端绑定逻辑会拒绝无效邀请码、自邀请和重复绑定；绑定失败不阻断注册，只写普通应用日志。
- 订阅和余额充值的支付宝订单会进入 `applyAffiliateRebateForOrder()`，按 `amount` 而非含手续费 `pay_amount` 返利。
- 余额支付订单不返利，符合当前设计。
- 发现并修复一处实质漏返 bug：`affiliateRebateBaseAmount()` 已允许 `traffic_pack`，但 `fulfillTrafficPackOrderInTx()` 原先没有调用 `applyAffiliateRebateForOrder()`，导致被邀请用户购买支付宝 GPT 流量包后不会返利。

## 本地代码修复

- `backend/internal/service/payment_fulfillment.go`
  - 在流量包发货成功后、订单标记完成前调用 `applyAffiliateRebateForOrder()`。
  - 由于 `affiliateRebateBaseAmount()` 只允许 `payment_type=alipay`，余额支付流量包仍不会返利。
- `backend/internal/service/payment_fulfillment_test.go`
  - 新增 `TestExecuteTrafficPackFulfillmentAppliesAffiliateRebateForAlipay`。
  - 已按 TDD 验证：修复前失败，失败原因是返利调用数为 `0`；修复后通过。

## 验证

- `go test -count=1 -tags=unit ./internal/service -run TestExecuteTrafficPackFulfillmentAppliesAffiliateRebateForAlipay`
- `go test -count=1 -tags=unit ./internal/service -run 'TestExecuteTrafficPackFulfillment|TestAffiliateRebateBaseAmount|TestExecuteSubscriptionFulfillment.*Affiliate'`
- `go test -count=1 -tags=unit ./internal/service`
- `pnpm test:run src/utils/__tests__/oauthAffiliate.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/views/auth/__tests__/EmailVerifyView.spec.ts src/views/auth/__tests__/OAuthCallbackView.spec.ts src/api/__tests__/auth-oauth-adoption.spec.ts`

## 部署状态

- 生产数据已修正。
- 后端流量包漏返修复目前只在本地工作树，尚未发布到公网 18084。

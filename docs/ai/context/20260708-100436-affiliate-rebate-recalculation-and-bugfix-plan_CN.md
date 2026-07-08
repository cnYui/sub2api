# 邀请返利重算与漏返审计执行计划

## 目标

- 把公网 18084 中 `2026-07-07` 已产生的旧 20% 邀请返利撤销，并按当前生产设置 `8%`、冻结 `24h`、单个被邀请用户累计上限 `100` 元重新入账。
- 审计并修复“邀请链接注册后，消费却没有返现”的代码风险。

## 数据修正方案

1. 只操作当前公网数据库 `sub2api-candidate-postgres/sub2api`。
2. 写库前备份 PostgreSQL dump 到 `deploy/backups/`，权限 `600`。
3. 修正范围只包含昨天已经识别的两笔返利相关订单：
   - `payment_orders.id=135`：`zhudi189@gmail.com` 的 79 元订阅，邀请人 `1510623550@qq.com`。
   - `payment_orders.id=139`：`961109198@qq.com` 的 99 元订阅，邀请人 `1038686518@qq.com`。
4. 事务内撤销旧流水：
   - 删除两笔旧 `user_affiliate_ledger(action='accrue', source_order_id in (135,139))`。
   - 删除 `1038686518@qq.com` 昨天把旧返利 `19.80` 转余额的 `transfer` 流水。
   - 从邀请人累计字段中扣回旧返利；已转余额的旧返利同步从 `users.balance` 扣回。
5. 事务内按当前规则重入账：
   - 79 元订单返利 `79 * 8% = 6.32`。
   - 99 元订单返利 `99 * 8% = 7.92`。
   - 两个被邀请用户历史累计都低于 `100`，不触发截断。
   - 按当前冻结设置写入 `aff_frozen_quota`，冻结起算点使用原订单返利时间，避免手工修正导致用户多等一天。
6. 同步更新 `payment_audit_logs` 中对应订单的 `AFFILIATE_REBATE_APPLIED` detail，避免后台记录仍显示旧金额。
7. 修正后查询邀请人快照、流水、订单审计记录，确认没有旧金额残留。

## 代码审计和修复方案

1. 后端注册链路：
   - 普通邮箱注册和邮箱验证码注册会把 `RegisterRequest.aff_code` 传给 `AuthService.RegisterWithVerification()`。
   - OAuth/pending 流程会保存并透传 `aff_code`。
2. 后端绑定链路：
   - `AffiliateService.BindInviterByCode()` 会创建被邀请人 affiliate profile、拒绝无效码、自邀请和重复绑定；绑定失败不阻断注册。
3. 后端支付返利链路：
   - 订阅订单会调用 `applyAffiliateRebateForOrder()`。
   - 余额支付不返利符合当前设计。
   - 已发现风险：`affiliateRebateBaseAmount()` 已允许 `traffic_pack`，但 `fulfillTrafficPackOrderInTx()` 没有调用 `applyAffiliateRebateForOrder()`，导致邀请用户购买 GPT 流量包可能不返利。
4. TDD 修复：
   - 先在 `backend/internal/service/payment_fulfillment_test.go` 新增失败测试，断言支付宝 `traffic_pack` 完成履约后会创建邀请返利流水和 `AFFILIATE_REBATE_APPLIED` 审计。
   - 运行目标测试确认失败。
   - 在 `fulfillTrafficPackOrderInTx()` 完成流量包发货后、标记完成前调用 `applyAffiliateRebateForOrder()`。
   - 运行目标测试和相关支付履约测试。

## 验证命令

- 数据修正前后：
  - `docker exec sub2api-candidate-postgres psql ...` 查询 `user_affiliate_ledger`、`user_affiliates`、`users.balance`、`payment_audit_logs`。
- 代码验证：
  - `go test -count=1 -tags=unit ./internal/service -run 'TestExecuteTrafficPackFulfillment.*Affiliate|TestExecuteSubscriptionFulfillment.*Affiliate'`
  - 如目标测试不足，再运行 `go test -count=1 -tags=unit ./internal/service`。

## 预期结果

- `1510623550@qq.com` 昨天返利从 `15.80` 调整为冻结 `6.32`。
- `1038686518@qq.com` 昨天已转余额旧返利 `19.80` 被扣回，改为冻结 `7.92`。
- 两笔订单审计 detail 改为 `rebateAmount=6.32/7.92`。
- 支付宝流量包购买也会触发邀请返利，避免“链接注册后购买流量包不返现”的漏返。

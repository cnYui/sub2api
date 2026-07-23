# 当前所有用户 10 USD 流量卡批量发放结果

## 结论

- 已给当前所有 active 用户发放一张 10 USD GPT/OpenAI 流量卡。
- 本次按系统现有商品 `GPT 流量包 10 刀` 执行，而不是新增一个 10 元人民币商品。
- 赠送订单金额为 0，不计入真实支付收入，不修改用户余额、订阅或 API Key。

## 批次

- 批次号：`grant-20260722-10usd-current-users`
- 订单号前缀：`grant-20260722-10usd-current-users-u`
- 流量卡：`traffic_packs.id=2 / gpt_traffic_10usd_3cny`
- 每用户额度：`10 USD`
- 平台：`openai`
- 有效期：365 天
- 到期时间：`2027-07-22 15:24:05.676367 +08`

## 发放范围

- 口径：`users.deleted_at IS NULL AND users.status='active'`
- 目标用户数：119
- 普通用户：117
- 管理员：2
- 没有发给软删除或非 active 用户。

## 备份

- 备份文件：`backups/20260722-162105-current-users-10usd-traffic-pack-prechange.sql`
- 大小：约 2.68 MB
- 验证：可读，包含 6741 条相关 INSERT。

## 写入结果

- `payment_orders`：119 条
- `user_traffic_credits`：119 条
- `traffic_credit_ledger` purchase 流水：119 条
- `payment_audit_logs`：119 条

聚合金额：

- 订单 `amount` 合计：0
- 订单 `pay_amount` 合计：0
- `initial_usd` 合计：1190 USD
- `remaining_usd` 合计：1190 USD
- purchase ledger 合计：1190 USD

## 验证

- active 用户缺失发放：0
- 非 active 或软删除误发：0
- 本批次订单用户数：119
- 本批次去重用户数：119
- 每条额度记录均有对应订单与 purchase ledger。

## 回滚提醒

- 若后续发现口径错误且本批次尚未消费，可按批次订单号删除本批次 ledger、credit、audit、order。
- 若已经产生 deduction ledger 或 usage logs，不能直接删本批次事实，需要先做计费影响核对。

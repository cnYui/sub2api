# 余额与流量卡欠费连续结算

时间：2026-08-19

## 背景

原实现会在普通余额欠费时将余额套餐置为 `debt_paused`，后续周额度不再刷新；流量卡只允许 OpenAI 请求使用，因此余额欠费后其他渠道无法切换到已购买的流量卡。

## 结算规则

1. 余额套餐首期和后续周额度不再因余额为负暂停。刷新时先以本周额度抵消普通余额欠费，剩余部分才成为本周可用额度。
2. 普通余额已经欠费的用户，任意渠道的后续用量统一从用户级流量卡额度池扣减，不区分 OpenAI、Anthropic、Gemini、Grok 或 Antigravity。
3. 流量卡不足以覆盖该用量时，先扣尽现有流量卡，未覆盖金额写入 `traffic_credit_debt_ledger`。普通余额不会因此继续扩大欠费。
4. 普通余额和流量卡均无净可用额度时，准入返回余额不足；用户需要等待下一周余额套餐刷新，或充值流量卡。
5. 流量卡充值在同一事务中锁定用户，优先写入流量卡欠费还款账本；只有抵债后的剩余金额进入可用流量卡额度。重复支付回调仍按订单幂等。

首次余额不足但请求已经由上游完成时，仍保留既有普通余额透支结算，用于完整记录该次已发生用量；从普通余额处于欠费状态的下一笔请求开始切换到流量卡。

## 数据迁移

新增 `backend/migrations/209_balance_and_traffic_debt_continuation.sql`：

- 创建流量卡欠费账本及索引。
- 将有效的历史 `debt_paused` 余额套餐恢复为 `active`。
- 将流量卡套餐和用户流量卡记录的平台更新为 `all`，并把可用额度索引改为用户级。
- 将在售流量卡名称和说明更新为全渠道额度池。

未修改已执行的 `198_traffic_packs.sql`，保证已有环境迁移校验不受影响；新库依次执行迁移后会由 `209` 完成最终规范化。

## 验证

已通过：

```powershell
go test -tags=unit ./internal/repository -run "Test(TrafficPack|ApplyUsageBilling|DeductUsageBilling)" -count=1
go test -tags=unit ./internal/service -run "Test(CreditInitialBalance|CreditDueBalances|DebtPaused|ResumeDebt|CheckBillingEligibility|CheckFreshBalanceDebt|CanUseTrafficPackCredit)" -count=1
```

覆盖余额套餐欠费刷新、Anthropic 等非 OpenAI 渠道的流量卡扣费、流量卡不足的欠费记录、充值全额/部分抵债、回调幂等和双重欠费准入。

# 到期用户请求未被中断的代码级原因

## 结论

- `xunskyler@gmail.com/users.id=19` 的套餐到期后仍能请求，不是因为过期套餐仍有效，而是鉴权在取不到 active subscription 后进入“余额/流量卡兜底”路径。
- 当前 OpenAI 流量卡准入只检查是否存在任意 `remaining_usd > 0 AND expires_at > now()` 的批次，不检查本次请求可能费用，也没有最低可用额度、预授权或额度预留。
- `0.00111155 USD` 的残余流量卡足以通过请求前准入，但不足以覆盖一次 `gpt-5.6-sol` 响应的真实 `ActualCost`。
- OpenAI 网关在上游响应完成后才异步提交 `RecordUsage`；`usage_billing_repo` 发现流量卡不足覆盖完整费用时返回 `ErrInsufficientBalance`，`RecordUsage` 直接返回错误，后面的 `usage_logs` 写入不会执行。
- 因此链路表现为：客户端已收到 HTTP 200，随后异步记账失败，费用未扣除，`usage_logs` 无明细。这是财务一致性漏洞，不只是 API Key active。

## 代码链路

1. 鉴权阶段只取 active subscription。

   - `backend/internal/server/middleware/api_key_auth_core.go:85` 到 `100`：只有 API Key 分组是 subscription type 时才调用 `GetActiveSubscription()`；找不到 active subscription 时 `subscription` 保持 `nil`。
   - `backend/internal/repository/user_subscription_repo.go:88` 到 `96`：active subscription 条件包含 `status = active` 和 `expires_at > time.Now()`。
   - 过期订阅不会放入请求 context，所以后续不是“继续扣套餐”，而是退到非订阅账单路径。

2. 请求前准入允许流量卡兜底。

   - `backend/internal/handler/openai_gateway_handler.go:296` 到 `316`：OpenAI 请求在转发前调用 `checkGatewayBillingEligibility()`。
   - `backend/internal/service/billing_cache_service.go:751` 到 `777`：订阅模式检查失败或余额模式检查失败时，会调用 `canUseTrafficPackCredit()` 尝试兜底。
   - `backend/internal/service/billing_cache_service.go:801` 到 `807`：兜底只要求 `trafficPackService.HasAvailableCredit()` 返回 true。
   - `backend/internal/repository/traffic_pack_repo.go:89` 到 `100`：`HasAvailableCredit()` 的 SQL 只查是否存在 `remaining_usd > 0` 且未过期的 OpenAI 流量卡。
   - 对照余额路径，`backend/internal/service/billing_cache_service.go:909` 到 `932` 已有 `balanceBelowEligibilityThreshold()` 和 `MinimumBalanceReserve`；流量卡没有对应保护。

3. 真实扣费发生在响应之后。

   - `backend/internal/handler/openai_gateway_handler.go:510` 到 `525`：OpenAI 成功转发后，把 `RecordUsage()` 提交给 usage worker。
   - `backend/internal/handler/openai_gateway_handler.go:1730` 到 `1735`：普通 OpenAI 请求走异步 `submitUsageRecordTask()`；只有图片结果走 mandatory fallback。
   - `backend/internal/service/openai_gateway_service.go:6130` 到 `6138`：`RecordUsage()` 再次通过 `shouldBillWithTrafficPack()` 判断是否改用流量卡。
   - `backend/internal/service/gateway_service.go:8896` 到 `8907`：`shouldBillWithTrafficPack()` 仍然只看 `HasAvailableCredit()`，不判断流量卡总额能否覆盖 `cost.ActualCost`。

4. 扣费失败会阻止 usage 落库。

   - `backend/internal/service/openai_gateway_service.go:6253` 到 `6268`：先执行 `applyUsageBilling()`。
   - `backend/internal/service/openai_gateway_service.go:6270` 到 `6273`：`billingErr != nil` 时直接返回，只有无错误才调用 `writeUsageLogBestEffort()`。
   - `backend/internal/service/gateway_service.go:9547` 到 `9564` 的通用链路也是同样顺序：先 billing，后 usage log。
   - `backend/internal/repository/usage_billing_repo.go:126` 到 `133`：流量卡扣费未覆盖完整费用时返回 `service.ErrInsufficientBalance`。
   - `backend/internal/repository/usage_billing_repo.go:198` 到 `205`：扣费事务里同样只锁定 `remaining_usd > 0` 的未过期批次，再由 `PlanTrafficCreditDeductions()` 判断是否覆盖完整金额。

## 为什么会出现“到期后无法中断”

准确说不是“到期后必须中断但没有中断”，而是当前产品语义允许“无套餐但有 OpenAI 流量卡”继续请求。这个语义本身可以成立，问题在于流量卡准入把“存在残额”误当成“可为本次请求付费”。

对非流式或流式大模型请求，准确费用只有响应完成后才知道。请求前只检查 `remaining_usd > 0`，必然会产生 TOCTOU 窗口：小额残余放行大额请求，响应返回后才发现扣不起。

## 验证

已运行现有单测确认当前语义：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestBillingCacheServiceAllowsOpenAITrafficPackWhenBalanceEmpty|TestPlanTrafficCreditDeductions_ReturnsUncoveredWhenBalanceInsufficient|TestShouldBillWithTrafficPackWhenSubscriptionRequestWouldExceedLimit'
```

结果：`ok github.com/Wei-Shaw/sub2api/internal/service`。

这些测试证明：

- 余额为 0 但 OpenAI 流量卡存在时，请求前准入允许通过。
- 流量卡总额不足完整请求费用时，扣费计划会返回 uncovered。
- 套餐即将或已经超额时，只要存在流量卡，代码会倾向切到流量卡账单。

## 修复方向

不要只把 API Key 到期或停用作为根修复；那只能处理个别用户，不能修复计费漏洞。

优先级建议：

1. 建立不可变 usage fact / durable outbox：上游响应一旦完成，token、模型、request_id、api_key_id、user_id、费用计算输入必须先持久化；扣费失败时 fact 保留为 `billing_failed/debt`，不能因为账务失败丢失 usage 明细。
2. 财务任务不能被普通内存队列丢弃：普通请求的 `RecordUsage` 至少要 mandatory sync fallback，长期应改成 durable queue；队列不可用时要 fail-closed 或进入可重放 outbox。
3. 流量卡准入要从 `HasAvailableCredit()` 改成可支付能力判断：至少增加最低可用额度阈值，根本方案是请求前预留预算，响应后精确结算并释放或补债。
4. 对 streaming / 大模型请求增加预算保护：仅靠“请求前余额大于 0”无法控制长输出，应支持最大费用预估、保守预留、超额债务和后续封禁。
5. 增加回归测试：过期订阅 + 余额 0 + 流量卡低于阈值应被请求前拒绝；流量卡短缺时必须保留 usage fact；usage worker 队列满时不能静默丢财务任务。

## 本次操作

- 仅读取源码、历史上下文和运行现有单元测试。
- 未修改业务代码、数据库、Redis、容器、Nginx、用户、套餐、API Key 或额度。
- 新增本文档用于记录代码级根因。

# 负余额硬拦截与欠费套餐暂停实现

## 背景与根因

历史链路允许请求先返回、再由 worker 异步结算。高频请求会在前一笔扣费尚未落库时继续通过旧余额准入；同时订阅、OpenAI 流量卡、SimpleMode、WebSocket 长连接和部分 Live sideband 入口存在不同程度的旁路。余额套餐周期任务还会自动使用下一周额度偿还欠费，与“首周不足后暂停、等待人工处理”的业务规则冲突。

本次修复的边界是：已经准入并到达上游的单笔或并发请求仍可能因最终费用未知而在结算时把余额从正数扣成负数，但结算完成后任何新请求都必须被数据库欠费事实阻断。若要保证余额永不出现负数，需要额外引入请求前资金预授权或最大消费冻结，不能用静态次数或平均费用替代。

## 实现

### 数据库欠费终检

- `BillingCacheService.CheckBillingEligibility` 在订阅、余额、流量卡和 RPM 判断前直读 PostgreSQL；`users.balance < 0` 统一返回 `INSUFFICIENT_BALANCE`。
- `users.balance = 0` 时仍可按既有规则使用足额 OpenAI 流量卡。
- SimpleMode 只跳过常规计费限制，不再跳过负余额终检。
- API Key、Gemini、OpenAI/Anthropic/Grok 等 handler、WebSocket 首轮/后续轮和 Live sideband 都纳入欠费拦截。
- Redis 不再作为欠费最终事实。只有仓库未注入的单元测试或简化组件才回退到缓存/用户快照；生产 Wire 始终注入数据库仓库。

### 同步资金结算与 fail-closed

- 网关使用量结算改为同步执行，handler 或 WebSocket 轮次在资金事务完成前不释放。
- 标准模式缺少统一 `UsageBillingRepository` 时返回 `BILLING_SERVICE_UNAVAILABLE`，不再降级到不维护套餐子账本的旧扣费路径。
- 扣费后继续失效余额缓存，下一请求以数据库余额重新终检。

### 明确的 `debt_paused` 套餐状态

- 首周额度仍先抵扣已有负余额；首周后仍为负数时，套餐立即进入 `debt_paused`。
- `debt_paused` 保留未消费的期数、原计划刷新时间和到期时间，但周期任务不再发放后续周额度。
- 迁移 `204_pause_negative_balance_packages.sql` 将上线时仍为 `active`、余额为负且还有后续期数的套餐幂等改为 `debt_paused`，并写支付审计。
- 管理员恢复接口为 `POST /api/v1/admin/payment/balance-packages/:id/resume-debt-paused`；只允许余额已不为负、套餐未到期且仍有后续期数时恢复，不延长有效期。
- 用户订阅页显示“欠费暂停”和原计划刷新时间，隐藏再次购买入口。

### 批量生图冻结来源

- 迁移 `205_batch_image_balance_package_source.sql` 为 `batch_image_jobs` 增加 `balance_package_id` 和 `balance_package_hold_usd`。
- 冻结普通余额时同步减少套餐 `remaining_usd`，捕获或释放时按来源恢复。
- 有效套餐的未消费冻结额先抵扣期间形成的负余额，只有剩余部分回到套餐子账本。
- 套餐在冻结期间到期时，不把到期套餐来源转换为永久普通余额，只退还普通余额来源。

## 验证

- 欠费终检、SimpleMode、API Key/Gemini、同步结算、统一仓库 fail-closed、WebSocket 每轮终检、`debt_paused` 生命周期和批量生图来源专项测试通过。
- `frontend` 的订阅页测试 3/3 通过，`pnpm typecheck` 与 `pnpm build` 通过。
- `go test ./internal/handler -count=1` 通过。
- `go test ./internal/service -count=1` 仅保留两个已确认的基线失败：`TestContentModerationRuntimeSnapshotRefreshFailureKeepsStaleConfig` 和依赖外部 OpenAI API 的 `TestEstimateOpenAIInputTokens_CompareWithOpenAIAPI`；本次相关测试无新增失败。

## 发布约束

- 只替换 `sub2api-official-18082` 应用容器。
- PostgreSQL、Redis、数据卷、Nginx 和 Cloudflare Tunnel 不重建。
- 公网验证前必须先执行 `docker exec sub2api-public-nginx-local nginx -t`。

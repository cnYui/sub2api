# Dashboard 套餐额度实时展示实施计划

> **面向执行代理：** 必须逐项执行并勾选。推荐使用 subagent-driven-development；也可使用 executing-plans 按检查点执行。

**目标：** 将 Dashboard 消费卡替换为实时的“今日使用 / 每日套餐额度”和“本订阅周期使用 / 本期理论额度”，并以不可变权益周期事实保证续费、退款和历史数据的口径正确。

**架构：** 新增 subscription_entitlement_periods 作为权益发放时的事实表。未来权益在同一事务内更新 user_subscriptions 并写入周期和额度快照。初次 Dashboard stats 返回完整统计加 quota，后续以轻量 quota-only 投影刷新。消费分子由成功 usage_facts 与不存在同键 fact 的历史 usage_logs 合并，因此同时覆盖套餐、余额和流量卡，且不会在异步结算后重复。

**技术栈：** Go、Ent、PostgreSQL、Gin、Vue 3、TypeScript、Vue I18n、Vitest、Testify。

---

## 已锁定边界

- 精确周期显示“今日使用 / 每日额度”和“本期使用 / 每日额度 x 本期天数”；分子取 actual_cost，允许超过分母。
- 同套餐提前续费从旧 expires_at 排入下一个周期，当前周期到期前 Dashboard 不切换。
- 无有效套餐时，今日和最近 30 天分母均显示 0；流量卡和余额消费仍计入分子。
- 历史表没有不可变 daily_limit_usd 快照。迁移不读取当前 groups.daily_limit_usd 伪造历史事实；无精确周期但订阅 active 时返回 rolling_30d_legacy。
- 后台负向调整和负天数兑换会破坏精确周期结束时间。它们撤销尚未过期的精确周期并降级为 legacy，不改写历史起止时间。
- 不修改实际扣费、流量卡预授权、套餐购买限制、退款金额、退款网关状态机或运行态。共享订阅、期限变化的退款继续人工审核。

## 文件结构

- Create: backend/migrations/166_subscription_entitlement_periods.sql
- Create: backend/migrations/167_dashboard_quota_usage_facts_index_notx.sql
- Create: backend/ent/schema/subscription_entitlement_period.go
- Create: backend/internal/service/subscription_entitlement_period.go
- Create: backend/internal/repository/subscription_entitlement_period_repo.go
- Create: backend/internal/repository/subscription_entitlement_period_repo_integration_test.go
- Create: backend/internal/repository/user_dashboard_quota_integration_test.go
- Create: backend/internal/handler/usage_handler_dashboard_test.go
- Create: backend/internal/service/redeem_service_subscription_entitlement_test.go
- Modify: backend/ent/schema/user.go, backend/ent/schema/group.go, backend/ent/schema/user_subscription.go
- Modify: backend/internal/service/subscription_service.go, backend/internal/service/user_subscription_port.go
- Modify: backend/internal/repository/user_subscription_repo.go, backend/internal/repository/usage_log_repo.go, backend/internal/repository/wire.go
- Modify: backend/internal/pkg/usagestats/usage_log_types.go
- Modify: backend/internal/service/account_usage_service.go, backend/internal/service/usage_service.go, backend/internal/service/wire.go
- Modify: backend/internal/service/payment_fulfillment.go, backend/internal/service/payment_balance_pay.go, backend/internal/service/payment_refund.go, backend/internal/service/payment_refund_state.go, backend/internal/service/redeem_service.go, backend/internal/service/auth_service.go, backend/internal/service/auth_oauth_first_bind.go, backend/internal/service/admin_service.go
- Modify: backend/internal/handler/usage_handler.go, backend/internal/handler/admin/subscription_handler.go, backend/internal/server/routes/user.go, backend/internal/server/api_contract_test.go
- Modify: backend/internal/repository/migrations_schema_integration_test.go, backend/internal/repository/usage_log_repo_integration_test.go
- Modify: backend/internal/service/payment_fulfillment_test.go, backend/internal/service/payment_balance_pay_test.go, backend/internal/service/payment_refund_test.go, backend/internal/service/subscription_assign_idempotency_test.go, backend/internal/service/subscription_revoke_test.go, backend/internal/service/auth_service_register_test.go
- Modify: frontend/src/api/usage.ts, frontend/src/components/user/dashboard/UserDashboardStats.vue, frontend/src/views/user/DashboardView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
- Modify: frontend/src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts, frontend/src/views/user/__tests__/DashboardView.spec.ts
- Generated: backend/ent/ 下的 Ent 代码和 backend/cmd/server/wire_gen.go

### Task 1: 建立权益周期事实表与 Ent 模型

**Files:**
- Create: backend/migrations/166_subscription_entitlement_periods.sql
- Create: backend/migrations/167_dashboard_quota_usage_facts_index_notx.sql
- Create: backend/ent/schema/subscription_entitlement_period.go
- Modify: backend/ent/schema/user.go
- Modify: backend/ent/schema/group.go
- Modify: backend/ent/schema/user_subscription.go
- Modify: backend/internal/repository/migrations_schema_integration_test.go

- [ ] **Step 1: 写入 schema 的失败断言**

在 TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate 中追加：

~~~go
requireColumn(t, tx, "subscription_entitlement_periods", "source_type", "character varying", 40, false)
requireColumn(t, tx, "subscription_entitlement_periods", "source_id", "character varying", 128, false)
requireColumn(t, tx, "subscription_entitlement_periods", "daily_limit_usd", "numeric", 0, true)
requireColumn(t, tx, "subscription_entitlement_periods", "period_days", "integer", 0, false)
requireIndex(t, tx, "subscription_entitlement_periods", "idx_subscription_entitlement_periods_source")
requireIndex(t, tx, "subscription_entitlement_periods", "idx_subscription_entitlement_periods_active_user_expiry")
requireIndex(t, tx, "usage_facts", "idx_usage_facts_dashboard_user_completed")
~~~

- [ ] **Step 2: 运行 migration 测试确认失败**

Run:

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate
~~~

Expected: FAIL，目标表或索引不存在。

- [ ] **Step 3: 添加表、约束和迁移索引**

在 166 migration 建立事实表；不要写历史回填 SQL：

~~~sql
CREATE TABLE IF NOT EXISTS subscription_entitlement_periods (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    subscription_id BIGINT NOT NULL REFERENCES user_subscriptions(id) ON DELETE RESTRICT,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    source_type VARCHAR(40) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    period_days INTEGER NOT NULL,
    daily_limit_usd NUMERIC(20,10),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    revoked_at TIMESTAMPTZ,
    revoked_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT subscription_entitlement_periods_days_check CHECK (period_days > 0),
    CONSTRAINT subscription_entitlement_periods_range_check CHECK (expires_at > starts_at),
    CONSTRAINT subscription_entitlement_periods_limit_check CHECK (daily_limit_usd IS NULL OR daily_limit_usd >= 0),
    CONSTRAINT subscription_entitlement_periods_status_check CHECK (status IN ('active', 'revoked'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_entitlement_periods_source
    ON subscription_entitlement_periods (source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_subscription_entitlement_periods_active_user_expiry
    ON subscription_entitlement_periods (user_id, expires_at, starts_at DESC)
    WHERE status = 'active';
~~~

167 是 non-transactional migration，内容只包含：

~~~sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_facts_dashboard_user_completed
    ON usage_facts (user_id, completed_at)
    INCLUDE (request_id, api_key_id)
    WHERE billing_status IN ('pending', 'settling', 'settled', 'debt');
~~~

- [ ] **Step 4: 定义 Ent schema 并生成代码**

新增 SubscriptionEntitlementPeriod schema，使用时间 mixin、无需 soft delete。通过 user_id、subscription_id、group_id 建必需边，并在 User、Group、UserSubscription 追加反向边。

~~~go
field.Float("daily_limit_usd").
    Optional().
    Nillable().
    SchemaType(map[string]string{dialect.Postgres: "numeric(20,10)"}),
field.Int("period_days"),
field.String("source_type").MaxLen(40),
field.String("source_id").MaxLen(128),
field.String("status").MaxLen(20).Default("active"),
~~~

Run:

~~~bash
cd backend
go generate ./ent
~~~

- [ ] **Step 5: 重新验证并提交**

Run:

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate
git add migrations/166_subscription_entitlement_periods.sql migrations/167_dashboard_quota_usage_facts_index_notx.sql ent internal/repository/migrations_schema_integration_test.go
git commit -m "feat: add subscription entitlement period schema"
~~~

Expected: PASS；重复迁移后表、约束和索引均存在。

### Task 2: 用来源幂等的事务原语发放与撤销周期

**Files:**
- Create: backend/internal/service/subscription_entitlement_period.go
- Create: backend/internal/repository/subscription_entitlement_period_repo.go
- Create: backend/internal/repository/subscription_entitlement_period_repo_integration_test.go
- Create: backend/internal/service/subscription_entitlement_period_test.go
- Modify: backend/internal/service/subscription_service.go
- Modify: backend/internal/service/user_subscription_port.go
- Modify: backend/internal/repository/user_subscription_repo.go
- Modify: backend/internal/repository/wire.go
- Modify: backend/internal/service/wire.go

- [ ] **Step 1: 写入连续性、幂等和回滚的失败测试**

使用固定时钟，覆盖同 source 只写一行、提前续费连续、事务回滚不残留：

~~~go
require.Equal(t, first.ExpiresAt, second.StartsAt)
require.Equal(t, 30, second.PeriodDays)
require.InDelta(t, 19.0, *second.DailyLimitUSD, 0.0000001)
require.True(t, replayed)
require.Equal(t, first.ExpiresAt, replayedResult.Subscription.ExpiresAt)
~~~

- [ ] **Step 2: 运行目标测试确认失败**

Run:

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run TestSubscriptionEntitlementPeriod
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run TestGrantSubscriptionEntitlement
~~~

Expected: FAIL，因为领域类型、repository 和事务原语未实现。

- [ ] **Step 3: 定义领域契约和 Ent repository**

在 subscription_entitlement_period.go 定义：

~~~go
type SubscriptionEntitlementSource struct {
    Type string
    ID   string
}

type SubscriptionEntitlementPeriod struct {
    ID             int64
    UserID         int64
    SubscriptionID int64
    GroupID        int64
    Source         SubscriptionEntitlementSource
    StartsAt       time.Time
    ExpiresAt      time.Time
    PeriodDays     int
    DailyLimitUSD  *float64
    Status         string
    RevokedAt      *time.Time
    RevokedReason  string
}

type SubscriptionEntitlementGrantResult struct {
    Subscription *UserSubscription
    Period       *SubscriptionEntitlementPeriod
    Replayed     bool
}

type SubscriptionEntitlementPeriodRepository interface {
    GetBySource(ctx context.Context, source SubscriptionEntitlementSource) (*SubscriptionEntitlementPeriod, error)
    Create(ctx context.Context, period *SubscriptionEntitlementPeriod) error
    RevokeUnexpiredBySubscription(ctx context.Context, subscriptionID int64, now time.Time, reason string) error
    RevokeBySource(ctx context.Context, source SubscriptionEntitlementSource, now time.Time, reason string) error
}
~~~

同时定义 ErrSubscriptionEntitlementPeriodNotFound。repository 的所有读写经 clientFromContext(ctx, r.client) 使用当前 Ent transaction。GetBySource 的缺失返回这个领域错误，Create 的 source 唯一冲突由服务层解释为幂等重放。

- [ ] **Step 4: 收敛为事务内 grant/revoke 原语**

给 AssignSubscriptionInput 增加 EntitlementSource。AssignSubscription 与 AssignOrExtendSubscription 都调用同一个内部 grant，顺序固定为：锁定用户行、按 source 查已有 period、读取同组订阅、创建或续期订阅、拷贝 group.DailyLimitUSD、插入周期。

~~~go
func (s *SubscriptionService) grantSubscriptionEntitlementInTx(
    ctx context.Context,
    input *AssignSubscriptionInput,
) (*SubscriptionEntitlementGrantResult, error) {
    if existing, err := s.entitlementRepo.GetBySource(ctx, input.EntitlementSource); err == nil {
        return &SubscriptionEntitlementGrantResult{Period: existing, Replayed: true}, nil
    }

    // 同一事务锁住 users.id 后才读取并更新 user_subscriptions。
    // 新 period 的 starts_at 是 now 或旧 expires_at，expires_at 是 starts_at 加实际天数。
}
~~~

withSubscriptionUpdateTx 必须先检查 dbent.TxFromContext(ctx)。已有外层事务时直接执行回调，只有没有事务时才开启 Ent transaction。RevokeSubscription 在软删除前撤销该 subscription 未失效的 active period。不要将 period_days 用作 API 请求准入规则。

- [ ] **Step 5: 验证事务语义并提交**

Run:

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run TestSubscriptionEntitlementPeriod
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'Test(GrantSubscriptionEntitlement|AssignOrExtendSubscription|RevokeSubscription)'
git add internal/service internal/repository
git commit -m "feat: persist subscription entitlement periods atomically"
~~~

Expected: PASS；来源重放不延长订阅，失败不留下孤立 subscription 或 period。

### Task 3: 接入支付、兑换、后台、默认发放与退款撤权

**Files:**
- Modify: backend/internal/service/payment_fulfillment.go
- Modify: backend/internal/service/payment_balance_pay.go
- Modify: backend/internal/service/payment_refund.go
- Modify: backend/internal/service/payment_refund_state.go
- Modify: backend/internal/service/redeem_service.go
- Modify: backend/internal/service/auth_service.go
- Modify: backend/internal/service/auth_oauth_first_bind.go
- Modify: backend/internal/service/admin_service.go
- Modify: backend/internal/handler/admin/subscription_handler.go
- Modify: backend/internal/service/payment_fulfillment_test.go
- Modify: backend/internal/service/payment_balance_pay_test.go
- Modify: backend/internal/service/payment_refund_test.go
- Modify: backend/internal/service/subscription_assign_idempotency_test.go
- Modify: backend/internal/service/subscription_revoke_test.go
- Modify: backend/internal/service/auth_service_register_test.go
- Create: backend/internal/service/redeem_service_subscription_entitlement_test.go

- [ ] **Step 1: 先写各来源的失败测试**

覆盖支付新购/续费、余额支付、混合支付、兑换码重放、默认发放和退款边界：

~~~go
require.Equal(t, oldExpiresAt, entitlement.StartsAt)
require.Equal(t, "payment_order", entitlement.Source.Type)
require.Equal(t, strconv.FormatInt(order.ID, 10), entitlement.Source.ID)
periods, err := client.SubscriptionEntitlementPeriod.Query().
    Where(
        subscriptionentitlementperiod.SourceTypeEQ("redeem_code"),
        subscriptionentitlementperiod.SourceIDEQ(strconv.FormatInt(code.ID, 10)),
    ).All(ctx)
require.NoError(t, err)
require.Len(t, periods, 1)
require.Equal(t, "active", period.Status)
activeCount, err := client.SubscriptionEntitlementPeriod.Query().
    Where(
        subscriptionentitlementperiod.UserIDEQ(userID),
        subscriptionentitlementperiod.SubscriptionIDEQ(subscriptionID),
        subscriptionentitlementperiod.StatusEQ("active"),
        subscriptionentitlementperiod.ExpiresAtGT(now),
    ).Count(ctx)
require.NoError(t, err)
require.Zero(t, activeCount)
~~~

- [ ] **Step 2: 运行生命周期测试确认失败**

Run:

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'Test(ConfirmPaymentCompletesSubscriptionWhenAmountMatches|BalancePaySubscriptionRenewsSameActiveSubscription|RequestRefund|AssignSubscription|RevokeSubscription|AuthService.*Subscription|Redeem.*Entitlement)'
~~~

Expected: FAIL，因为入口尚未传递 source，退款也未同步撤销 period。

- [ ] **Step 3: 为每个发放入口传递稳定 source 并复用事务**

| 路径 | source_type | source_id |
| --- | --- | --- |
| 套餐支付、余额支付、混合支付 | payment_order | payment_orders.id |
| 正天数兑换码 | redeem_code | redeem_codes.id |
| 注册默认套餐 | signup_default | user_id:group_id |
| 首次绑定默认套餐 | provider_default | user_provider_default_grants 的稳定记录 ID 加 group ID |
| 管理员首次分配 | admin_assignment | user_id:group_id:validity_days:notes_sha256 |
| 管理员正向延长 | admin_adjustment | subscription_id:new_expires_at |

admin_assignment 的 notes_sha256 固定为 SHA-256(UTF-8(strings.TrimSpace(notes))) 的小写十六进制编码；source_id 使用 fmt.Sprintf("%d:%d:%d:%x", userID, groupID, normalizedValidityDays, digest)，不会超过 128 字符，并与现有“同用户、同组、同天数、同备注复用”的语义一致。

外部 fulfillSubscriptionOrderInTx 没有 TxFromContext 时先开履约事务，再完成 subscription grant、payment_orders.subscription_id、审计与订单完成。余额支付和兑换码已有外层事务时必须复用，不能嵌套 transaction。

- [ ] **Step 4: 落实撤权与破坏周期边界的调整**

支付退款只撤销当前 order 的 source：

~~~go
_ = s.entitlementRepo.RevokeBySource(ctx, SubscriptionEntitlementSource{
    Type: "payment_order",
    ID:   strconv.FormatInt(order.ID, 10),
}, now, "payment_refund")
~~~

管理员整项撤销撤销该 subscription 的全部未失效精确周期：

~~~go
_ = s.entitlementRepo.RevokeUnexpiredBySubscription(ctx, subscriptionID, now, "admin_revoke")
~~~

保留 validateExclusiveRefundSubscriptionWithClient 的共享订阅和期限变化人工审核分支。管理员负向延长和负天数兑换不截短历史事实行，撤销未失效精确周期并降级为 rolling_30d_legacy；管理员正向延长可追加从旧 expires_at 开始的 period。

- [ ] **Step 5: 运行来源回归并提交**

Run:

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
git add internal/service internal/handler/admin
git commit -m "feat: record subscription entitlement grant sources"
~~~

Expected: PASS；同套餐续费连续，流量卡和余额计费未变，退款人工审核保护仍有效。

### Task 4: 实现事实优先的 quota 读模型和轻量接口

**Files:**
- Modify: backend/internal/pkg/usagestats/usage_log_types.go
- Modify: backend/internal/service/account_usage_service.go
- Modify: backend/internal/service/usage_service.go
- Modify: backend/internal/repository/usage_log_repo.go
- Modify: backend/internal/handler/usage_handler.go
- Modify: backend/internal/server/routes/user.go
- Modify: backend/internal/server/api_contract_test.go
- Modify: backend/internal/repository/usage_log_repo_integration_test.go
- Create: backend/internal/repository/user_dashboard_quota_integration_test.go
- Create: backend/internal/handler/usage_handler_dashboard_test.go

- [ ] **Step 1: 写入 read model 和 HTTP 契约的失败测试**

固定 now，测试 pending fact、settled/debt fact 去重、单独历史 log、failed fact、用户隔离、上海跨零点、精确周期、无限额度、legacy、无套餐和超额：

~~~go
require.InDelta(t, 19.2, quota.TodayUsageUSD, 0.0000001)
require.InDelta(t, 570.0, quota.PeriodLimitUSD, 0.0000001)
require.Equal(t, "entitlement_period", quota.PeriodMode)
require.Greater(t, quota.TodayUsageUSD, quota.TodayLimitUSD)
~~~

handler 用真实 auth subject 断言 GET /usage/dashboard/quota 未认证为 401，认证用户只得到自己的 quota。

- [ ] **Step 2: 运行读模型测试确认失败**

Run:

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run 'Test(UserDashboardQuota|UsageLogRepoSuite)'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler -run TestUsageHandlerDashboardQuota
~~~

Expected: FAIL，因为 DTO、query 和 quota-only route 未存在。

- [ ] **Step 3: 定义固定返回 DTO 和窗口选择**

在 usagestats 定义下列字段，JSON 名称依次为 period_mode、today_usage_usd、today_limit_usd、period_usage_usd、period_limit_usd、period_starts_at、period_expires_at、period_days：

~~~go
type UserDashboardQuota struct {
    PeriodMode      string
    TodayUsageUSD   float64
    TodayLimitUSD   float64
    PeriodUsageUSD  float64
    PeriodLimitUSD  float64
    PeriodStartsAt  *time.Time
    PeriodExpiresAt *time.Time
    PeriodDays      *int
}
~~~

UserDashboardStats 持有非省略的 Quota。使用 time.LoadLocation("Asia/Shanghai") 和传入 now 算半开区间：今日 [00:00, next 00:00)；当前精确 period 为 starts_at <= now 且 expires_at > now；没有精确但存在 active subscription 时返回 rolling_30d_legacy；没有 active subscription 时返回 none 与两个零分母。

- [ ] **Step 4: 实现 fact/log 去重 SQL 和两种 API 投影**

UsageLogRepository 增加 GetUserDashboardQuota，GetUserDashboardStats 调用相同方法填充 stats.Quota。查询必须兼容当前 UsageFactPayload 中未加 JSON tag 的 UsageLog.ActualCost 和 effects.actual_cost：

~~~sql
WITH fact_costs AS (
  SELECT f.completed_at AS occurred_at,
    COALESCE(
      NULLIF(f.payload #>> '{usage_log,ActualCost}', '')::numeric,
      NULLIF(f.payload #>> '{effects,actual_cost}', '')::numeric,
      0
    ) AS actual_cost
  FROM usage_facts f
  WHERE f.user_id = $1
    AND f.billing_status IN ('pending', 'settling', 'settled', 'debt')
    AND f.completed_at >= $2 AND f.completed_at < $3
), legacy_log_costs AS (
  SELECT ul.created_at AS occurred_at, ul.actual_cost
  FROM usage_logs ul
  WHERE ul.user_id = $1
    AND ul.created_at >= $2 AND ul.created_at < $3
    AND NOT EXISTS (
      SELECT 1 FROM usage_facts f
      WHERE f.request_id = ul.request_id AND f.api_key_id = ul.api_key_id
    )
), all_costs AS (
  SELECT * FROM fact_costs UNION ALL SELECT * FROM legacy_log_costs
)
SELECT COALESCE(SUM(actual_cost) FILTER (WHERE occurred_at >= $4 AND occurred_at < $5), 0),
       COALESCE(SUM(actual_cost) FILTER (WHERE occurred_at >= $6 AND occurred_at < $7), 0)
FROM all_costs;
~~~

UsageService.GetUserDashboardQuota、UsageHandler.DashboardQuota 和 GET /usage/dashboard/quota 只调用该投影。初始 GET /usage/dashboard/stats 仍返回完整统计加相同 quota，轮询不扫描无时间上限的累计统计。

- [ ] **Step 5: 验证 quota endpoint 并提交**

Run:

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run 'Test(MigrationsRunner_IsIdempotent_AndSchemaIsUpToDate|UserDashboardQuota|UsageLogRepoSuite)'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
git add internal/pkg/usagestats internal/service internal/repository internal/handler internal/server
git commit -m "feat: add realtime dashboard quota projection"
~~~

Expected: PASS；initial stats 与 quota-only endpoint 使用同一读模型，实时 fact 不会被 worker 延迟隐藏。

### Task 5: 替换消费卡的数值展示和 API 类型

**Files:**
- Modify: frontend/src/api/usage.ts
- Modify: frontend/src/components/user/dashboard/UserDashboardStats.vue
- Modify: frontend/src/i18n/locales/zh.ts
- Modify: frontend/src/i18n/locales/en.ts
- Modify: frontend/src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts

- [ ] **Step 1: 写入消费卡失败断言**

在组件 fixture 中加入 quota，覆盖精确、legacy、无套餐和超额：

~~~ts
expect(wrapper.text()).toContain('今日使用')
expect(wrapper.text()).toContain('$0.0000 / $19.0000')
expect(wrapper.text()).toContain('本期使用')
expect(wrapper.text()).toContain('$580.0000 / $570.0000')
expect(wrapper.text()).not.toContain('标准')
~~~

- [ ] **Step 2: 运行组件测试确认失败**

Run:

~~~bash
pnpm --dir frontend exec vitest run src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts
~~~

Expected: FAIL，因为 quota 类型、文案和渲染未实现。

- [ ] **Step 3: 同步 API 类型、文案与组件**

在 usage.ts 声明：

~~~ts
export type DashboardQuotaPeriodMode = 'entitlement_period' | 'rolling_30d_legacy' | 'none'

export interface UserDashboardQuota {
  period_mode: DashboardQuotaPeriodMode
  today_usage_usd: number
  today_limit_usd: number
  period_usage_usd: number
  period_limit_usd: number
  period_starts_at?: string
  period_expires_at?: string
  period_days?: number
}
~~~

UserDashboardStats.quota 为必需字段。当前 today_actual_cost / today_cost、total_actual_cost / total_cost 两行替换为 quota。标签使用 dashboard.todayUsage；精确周期用 dashboard.subscriptionPeriodUsage，rolling_30d_legacy 与 none 用 dashboard.last30DaysUsage。继续用 formatCost，固定四位小数，禁止前端钳制分子。保留 actual、standard 键和原统计字段供其他页面使用。

- [ ] **Step 4: 运行组件测试、类型检查并提交**

Run:

~~~bash
pnpm --dir frontend exec vitest run src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts
pnpm --dir frontend typecheck
git add frontend/src/api/usage.ts frontend/src/components/user/dashboard/UserDashboardStats.vue frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts frontend/src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts
git commit -m "feat: show subscription quota on user dashboard"
~~~

Expected: PASS；无套餐显示 0 分母，超额保持真实值。

### Task 6: 让可见 Dashboard 只轮询额度投影

**Files:**
- Modify: frontend/src/api/usage.ts
- Modify: frontend/src/views/user/DashboardView.vue
- Modify: frontend/src/views/user/__tests__/DashboardView.spec.ts

- [ ] **Step 1: 写入轮询、可见性和卸载的失败测试**

用 vi.useFakeTimers 断言 15 秒后只有 quota 请求增加：

~~~ts
await vi.advanceTimersByTimeAsync(15_000)
expect(mockGetDashboardQuota).toHaveBeenCalledTimes(1)
expect(mockGetDashboardTrend).toHaveBeenCalledTimes(1)
expect(mockGetDashboardModels).toHaveBeenCalledTimes(1)
expect(mockGetByDateRange).toHaveBeenCalledTimes(1)

wrapper.unmount()
await vi.advanceTimersByTimeAsync(30_000)
expect(mockGetDashboardQuota).toHaveBeenCalledTimes(1)
~~~

同时模拟 hidden 时停表，恢复 visible 与连续 focus 只发一条 quota 请求。

- [ ] **Step 2: 运行页面测试确认失败**

Run:

~~~bash
pnpm --dir frontend exec vitest run src/views/user/__tests__/DashboardView.spec.ts
~~~

Expected: FAIL，因为没有 getDashboardQuota、timer 或事件清理。

- [ ] **Step 3: 实现无闪烁的 quota 刷新调度**

在 API client 增加：

~~~ts
export async function getDashboardQuota(): Promise<UserDashboardQuota> {
  const { data } = await apiClient.get<UserDashboardQuota>('/usage/dashboard/quota')
  return data
}
~~~

DashboardView 保留首次 refreshAll，但新增只更新 stats.value.quota 的 refreshQuota。用唯一 in-flight Promise 合并 visibilitychange 与 focus；仅 document.visibilityState 为 visible 时启动 15 秒 interval，hidden 停止，恢复后立即刷新再启动。onBeforeUnmount 必须停止 interval 并移除两个 listener。轮询不改全页 loading，绝不调用 authStore.refreshUser、loadCharts 或 loadRecent。

- [ ] **Step 4: 运行页面测试、类型检查、构建并提交**

Run:

~~~bash
pnpm --dir frontend exec vitest run src/views/user/__tests__/DashboardView.spec.ts
pnpm --dir frontend typecheck
pnpm --dir frontend build
git add frontend/src/api/usage.ts frontend/src/views/user/DashboardView.vue frontend/src/views/user/__tests__/DashboardView.spec.ts
git commit -m "feat: refresh dashboard quota while visible"
~~~

Expected: PASS；隐藏和卸载后无后台请求，轮询只访问轻量接口。

### Task 7: 全量回归、生成物检查和结果记录

**Files:**
- Modify: AGENTS.md
- Create: docs/ai/context/20260716-155244-dashboard-subscription-quota-realtime-result_CN.md

- [ ] **Step 1: 运行所有受影响层的回归**

Run:

~~~bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
cd ..
pnpm --dir frontend exec vitest run src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts src/views/user/__tests__/DashboardView.spec.ts
pnpm --dir frontend typecheck
pnpm --dir frontend build
git diff --check
~~~

Expected: 全部 PASS。未触及模块若有既有失败，结果文档必须列出测试名、模块和与本次无关的证据，不能写成通过。

- [ ] **Step 2: 检查生成物和 migration 执行模式**

Run:

~~~bash
cd backend
go generate ./ent
go generate ./cmd/server
git diff --check
git status --short
~~~

Expected: 167 migration 只含并发 index；wire 与 Ent 生成物同步；没有非预期 schema 漂移。

- [ ] **Step 3: 写入结果上下文并提交实现**

结果文档记录精确/legacy/none 口径、fact/log 去重、轮询频率和可见性规则、migration 编号、已运行命令与输出结论，以及未部署、未改运行态。AGENTS 最高优先级记忆须明确历史未回填是为了不伪造每日额度快照。

Run:

~~~bash
git add AGENTS.md docs/ai/context backend frontend
git commit -m "feat: add realtime subscription quota dashboard"
~~~

Expected: 只提交本功能文件；不修改生产数据库、Redis、容器、Nginx 或部署配置。

## 实施前自审

- 需求覆盖：Task 1-3 覆盖支付、续费、兑换码、后台、默认与退款；Task 4 覆盖实时 fact、历史 log、全部计费来源、时区和分母；Task 5-6 覆盖数值展示与 15 秒刷新。
- 边界一致性：历史缺快照、负向调整和人工审核不会产生伪精确周期；无套餐、仅流量卡和超额消费保持确认的展示口径。
- 接口一致性：UserDashboardQuota、GET /usage/dashboard/quota、前端 UserDashboardQuota 与组件字段同名；initial stats 与轮询复用同一个 read model。
- 范围控制：不改变实际扣费、流量卡预授权、套餐购买限制、退款金额、退款网关状态机或运行态。

# 后台分配周套餐订阅失败只读调查与修复设计

## 重要边界

- 用户新指令：本地先进行设计，不再触碰公网任何链路。
- 从本文件创建开始，调查仅限本地代码与文档，不再访问或修改公网 DB、Redis、容器、Nginx、Cloudflare、公网 API。
- 在该新指令之前，已发生一次运行态手动补发与 smoke；后续如需回滚必须另行明确授权，因为回滚同样属于运行态修改。

## 问题现象

- 管理后台“分配订阅”给 `tongji_lishouqi@163.com` 的 59 元套餐时报“分配订阅失败”。
- 59 元套餐当前是公共 Codex 周额度套餐：`codex-pool-49-usd`，28 天有效，按订阅锚点每 7 天滚动刷新额度，周额度 118 USD。
- 该用户历史上已有同分组旧订阅，但它是旧日额度/30 天语义下创建的记录，且没有 `subscription_entitlement_periods` 权益周期事实。

## 本地静态根因

### 1. 旧订阅记录挡住新分配

`backend/migrations/016_soft_delete_partial_unique_indexes.sql` 中唯一索引是：

```sql
CREATE UNIQUE INDEX IF NOT EXISTS user_subscriptions_user_group_unique_active
    ON user_subscriptions(user_id, group_id)
    WHERE deleted_at IS NULL;
```

它只排除软删除，不排除 `status='expired'`。因此同用户同分组只要有历史未删除订阅，后台分配不能创建第二条订阅，只能复用或更新旧订阅。

### 2. 管理员单人分配走错了旧幂等路径

入口：`backend/internal/handler/admin/subscription_handler.go`

```go
subscription, err := h.subscriptionService.AssignSubscription(c.Request.Context(), &service.AssignSubscriptionInput{
    UserID:       req.UserID,
    GroupID:      req.GroupID,
    ValidityDays: req.ValidityDays,
    AssignedBy:   adminID,
    Notes:        req.Notes,
})
```

服务：`backend/internal/service/subscription_service.go`

```go
func (s *SubscriptionService) AssignSubscription(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, error) {
    if hasSubscriptionEntitlementSource(input) {
        result, err := s.GrantSubscriptionEntitlement(ctx, input)
        ...
    }
    if shouldCreateAdminAssignmentEntitlement(input) {
        sub, _, err := s.assignSubscriptionWithAdminAssignmentEntitlement(ctx, input)
        return sub, err
    }

    sub, _, err := s.assignSubscriptionWithReuse(ctx, input)
    ...
}
```

`assignSubscriptionWithAdminAssignmentEntitlement` 内部先调用 `assignSubscriptionWithReuse`。而 `assignSubscriptionWithReuse` 一旦发现同用户同分组已有订阅，就直接按“幂等复用/冲突”处理，不会续期过期订阅，也不会创建 entitlement period。

### 3. 30 天旧记录与 28 天周套餐发生语义冲突

`detectAssignSemanticConflict` 对公共 Codex 周套餐会把有效期归一化为 28 天：

```go
if existing.Group != nil && existing.Group.UsesRollingWeeklyQuota() {
    normalizedDays = 28
}
expectedExpiresAt := existing.StartsAt.AddDate(0, 0, normalizedDays)
if !existing.ExpiresAt.Equal(expectedExpiresAt) {
    return "validity_days_mismatch", true
}
```

旧记录是 30 天，当前套餐是 28 天，所以后台分配可能直接返回 `SUBSCRIPTION_ASSIGN_CONFLICT`。前端只显示通用 toast：`分配订阅失败`。

### 4. 即使不冲突，也可能缺权益周期

现有测试 `TestBulkAssignSubscriptionCreatedReusedAndConflict` 明确期望：同语义已有订阅被 `reused` 时，不创建 `admin_assignment` entitlement period。

这在旧日额度模式下还能工作，因为限额主要读 `user_subscriptions + groups`；但周额度模式要求 `subscription_entitlement_periods.weekly_limit_usd / period_total_quota_usd / quota_window_unit` 作为不可变权益事实。复用旧记录而不建 period，会导致请求前授权/账本归属缺关键事实。

## 推荐修复方向

核心原则：不要新增手写 SQL 或第二套订阅状态机，复用已有的 `GrantSubscriptionEntitlement`，因为它已经处理：

- 同来源幂等 replay；
- 同用户串行锁；
- 已过期订阅从当前时间重新激活；
- 未过期订阅从当前过期时间续期；
- 周套餐写入 `weekly_limit_usd / period_total_quota_usd / quota_window_unit / quota_window_days`；
- 订阅缓存失效。

### 最小行为变更

只修改管理员分配路径，保留普通 `AssignSubscription` 的历史幂等语义。

对 `AssignedBy > 0 && entitlementPeriodRepo != nil` 的请求：

1. 若没有同用户同组订阅：创建订阅并创建 admin_assignment entitlement period。
2. 若已有同用户同组订阅且已过期：重新激活旧订阅并创建新的 admin_assignment entitlement period。
3. 若已有同用户同组订阅且仍有效：保持现有行为，语义一致返回 reused，语义冲突返回 `SUBSCRIPTION_ASSIGN_CONFLICT`，不自动叠加套餐。后台已有单独“延长/调整”入口，避免误把分配按钮变成续费按钮。
4. 对公共 Codex 周套餐，有效期固定按 28 天，不使用前端传来的 30 天旧值。

## 代码设计

### 修改文件

- `backend/internal/service/subscription_service.go`
- `backend/internal/service/subscription_assign_idempotency_test.go`

### 新增测试 1：过期旧周套餐订阅可重新分配并建权益周期

放在 `backend/internal/service/subscription_assign_idempotency_test.go`。

测试要点：

```go
func TestAssignSubscriptionAdminAssignmentRenewsExpiredRollingWeeklySubscription(t *testing.T) {
    now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
    weeklyLimit := 118.0
    groupRepo := &subscriptionGroupRepoStub{
        group: &Group{
            ID:               4,
            Name:             "codex-pool-49-usd",
            SubscriptionType: SubscriptionTypeSubscription,
            WeeklyLimitUSD:   &weeklyLimit,
        },
    }
    subRepo := newSubscriptionUserSubRepoStub()
    oldStart := now.AddDate(0, 0, -33)
    subRepo.seed(&UserSubscription{
        ID:             47,
        UserID:         27,
        GroupID:        4,
        StartsAt:       oldStart,
        ExpiresAt:      oldStart.AddDate(0, 0, 30),
        Status:         SubscriptionStatusExpired,
        DailyUsageUSD:  0,
        WeeklyUsageUSD: 90,
        Notes:          "old manual assignment",
        Group:          groupRepo.group,
    })
    entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
    svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
    svc.entitlementPeriodRepo = entitlementRepo
    svc.now = func() time.Time { return now }

    sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
        UserID:       27,
        GroupID:      4,
        ValidityDays: 28,
        AssignedBy:   1,
        Notes:        "manual reassign after weekly migration",
    })

    require.NoError(t, err)
    require.Equal(t, int64(47), sub.ID)
    require.Equal(t, SubscriptionStatusActive, sub.Status)
    require.Equal(t, now, sub.StartsAt)
    require.Equal(t, now.AddDate(0, 0, 28), sub.ExpiresAt)
    require.NotNil(t, sub.WeeklyAnchorAt)
    require.Equal(t, now, *sub.WeeklyAnchorAt)
    require.InDelta(t, 0, sub.WeeklyUsageUSD, 1e-9)

    source := adminAssignmentSubscriptionEntitlementSource(27, 4, 28, "manual reassign after weekly migration")
    period, err := entitlementRepo.GetBySource(context.Background(), source)
    require.NoError(t, err)
    require.Equal(t, int64(47), period.SubscriptionID)
    require.Equal(t, now, period.StartsAt)
    require.Equal(t, now.AddDate(0, 0, 28), period.ExpiresAt)
    require.NotNil(t, period.WeeklyLimitUSD)
    require.InDelta(t, 118, *period.WeeklyLimitUSD, 1e-9)
    require.NotNil(t, period.PeriodTotalQuotaUSD)
    require.InDelta(t, 472, *period.PeriodTotalQuotaUSD, 1e-9)
    require.Equal(t, "week", period.QuotaWindowUnit)
    require.Equal(t, 7, period.QuotaWindowDays)
}
```

预期红灯：

```bash
cd backend
go test -tags unit ./internal/service -run TestAssignSubscriptionAdminAssignmentRenewsExpiredRollingWeeklySubscription -count=1
```

当前代码会在 `validity_days_mismatch` 上返回 `SUBSCRIPTION_ASSIGN_CONFLICT`，或复用旧订阅但不创建权益周期。

### 新增测试 2：未过期订阅仍不被分配按钮自动续费

目的：防止修复过度，把“分配”变成“续期”。

```go
func TestAssignSubscriptionAdminAssignmentKeepsExistingActiveReuseSemantics(t *testing.T) {
    now := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
    weeklyLimit := 118.0
    groupRepo := &subscriptionGroupRepoStub{
        group: &Group{
            ID:               4,
            Name:             "codex-pool-49-usd",
            SubscriptionType: SubscriptionTypeSubscription,
            WeeklyLimitUSD:   &weeklyLimit,
        },
    }
    subRepo := newSubscriptionUserSubRepoStub()
    subRepo.seed(&UserSubscription{
        ID:        48,
        UserID:    27,
        GroupID:   4,
        StartsAt:  now,
        ExpiresAt: now.AddDate(0, 0, 28),
        Status:    SubscriptionStatusActive,
        Notes:     "same-note",
        Group:     groupRepo.group,
    })
    entitlementRepo := newSubscriptionEntitlementPeriodRepoStub()
    svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
    svc.entitlementPeriodRepo = entitlementRepo
    svc.now = func() time.Time { return now }

    sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{
        UserID:       27,
        GroupID:      4,
        ValidityDays: 28,
        AssignedBy:   1,
        Notes:        "same-note",
    })

    require.NoError(t, err)
    require.Equal(t, int64(48), sub.ID)
    require.Equal(t, now.AddDate(0, 0, 28), sub.ExpiresAt)
    require.Empty(t, entitlementRepo.periods)
}
```

### 最小实现

在 `assignSubscriptionWithAdminAssignmentEntitlement` 中新增“过期旧订阅优先走权益发放器”的分支。

伪代码：

```go
func (s *SubscriptionService) assignSubscriptionWithAdminAssignmentEntitlement(
    ctx context.Context,
    input *AssignSubscriptionInput,
) (*UserSubscription, bool, error) {
    if !shouldCreateAdminAssignmentEntitlement(input) || s.entitlementPeriodRepo == nil {
        return s.assignSubscriptionWithReuse(ctx, input)
    }

    group, err := s.groupRepo.GetByID(ctx, input.GroupID)
    if err != nil {
        return nil, false, fmt.Errorf("get admin assignment subscription group: %w", err)
    }
    if !group.IsSubscriptionType() {
        return nil, false, ErrGroupNotSubscriptionType
    }

    existingSub, err := s.userSubRepo.GetByUserIDAndGroupID(ctx, input.UserID, input.GroupID)
    if err != nil && !errors.Is(err, ErrSubscriptionNotFound) {
        return nil, false, err
    }
    if existingSub != nil && existingSub.ExpiresAt.After(s.currentTime()) {
        return s.assignSubscriptionWithReuse(ctx, input)
    }

    validityDays := input.ValidityDays
    if group.UsesRollingWeeklyQuota() {
        validityDays = 28
    }

    grantInput := *input
    grantInput.ValidityDays = validityDays
    grantInput.GroupSnapshot = group
    grantInput.EntitlementSource = adminAssignmentSubscriptionEntitlementSource(input.UserID, input.GroupID, validityDays, input.Notes)
    result, err := s.GrantSubscriptionEntitlement(ctx, &grantInput)
    if err != nil {
        return nil, false, err
    }
    return result.Subscription, result.Extended || result.Replayed, nil
}
```

需要在 `subscription_service.go` import 加 `errors`。

### 设计注意点

- 该方案会让“过期旧订阅”重新激活旧行，而不是创建新行；符合现有唯一索引，不需要迁移。
- 不改支付订单发放路径，因为 `payment_fulfillment.go` 已经显式使用 `EntitlementSource=payment_order`，会走 `GrantSubscriptionEntitlement`。
- 不改兑换码路径，因为 `redeem_service.go` 已经显式使用 `EntitlementSource=redeem_code`。
- 不建议修改唯一索引为 `status='active'`，因为同用户同组多条历史订阅会让读模型、退款、usage 归属和后台列表复杂化，属于大范围账本设计变更。
- 不建议前端 special-case 28 天，因为后端才是权益事实源；前端只负责默认值展示。

## 验证计划

```bash
cd backend
go test -tags unit ./internal/service -run "TestAssignSubscriptionAdminAssignmentRenewsExpiredRollingWeeklySubscription|TestAssignSubscriptionAdminAssignmentKeepsExistingActiveReuseSemantics|TestBulkAssignSubscriptionCreatedReusedAndConflict" -count=1
go test -tags unit ./internal/service -run "TestGrantSubscriptionEntitlement|TestAssignSubscription" -count=1
go test ./internal/service -run "TestGrantSubscriptionEntitlement|TestAssignSubscription" -count=1
```

如涉及 Ent schema 或迁移，不应在本修复里做；本问题可以在服务层完成。

## 后续运行态处理建议

- 本地代码修复验证通过后，再单独写运行态发布计划。
- 发布前检查当前公网事实源中所有 `status='expired' AND deleted_at IS NULL` 且属于公共 Codex 周套餐的旧订阅，评估是否需要一次性 backfill entitlement period。
- 对已经手动补发的用户，避免重复发放同一时间窗；应通过 `subscription_entitlement_periods.source_type/source_id` 或人工记录识别。

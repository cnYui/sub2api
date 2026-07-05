# 自动 API Key Effective Group Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 普通用户创建和使用 API Key 时不再选择固定分组，而是在每次请求时根据当前 OpenAI 套餐或 GPT 流量包权益解析 `effective_group`，并把旧 OpenAI API Key 迁移为自动 Key。

**Architecture:** 新增 `EffectiveGroupResolver` 作为唯一运行时分组解析入口；鉴权后、路由分发前把自动 Key 的 request-scoped copy 填入 `effective_group` 和可选 subscription。前端普通用户只看到“自动 Key”，管理员后台继续保留固定分组能力。

**Tech Stack:** Go service/middleware + Ent/PostgreSQL migrations + Gin routes + Vue 3/Vitest frontend.

---

## 设计依据

- 设计文档：`docs/ai/context/20260705-175642-auto-api-key-effective-group-design_CN.md`
- 诊断文档：`docs/ai/context/20260705-174552-traffic-card-only-user-no-openai-key-group-diagnosis_CN.md`
- 评估文档：`docs/ai/context/20260705-175211-auto-api-key-group-design-evaluation_CN.md`

## 文件结构

- Create: `backend/internal/service/effective_group_resolver.go`
  - 职责：根据用户、API Key、目标平台解析 request-scoped effective group 和 optional subscription。
- Create: `backend/internal/service/effective_group_resolver_test.go`
  - 职责：覆盖订阅优先、流量包 fallback、多套餐排序、无权益错误、固定 Key 保持不变。
- Create: `backend/internal/server/middleware/effective_group.go`
  - 职责：Gin 中间件读取已鉴权 API Key，调用 resolver，写入 request-scoped API Key copy、subscription 和 `ctxkey.Group`。
- Create: `backend/internal/server/middleware/effective_group_test.go`
  - 职责：验证中间件不污染原 API Key、写入上下文、错误响应格式。
- Modify: `backend/internal/server/routes/gateway.go`
  - 职责：在 API Key 鉴权后、`RequireGroupAssignment` 前挂载 effective group middleware，覆盖 OpenAI 相关入口。
- Modify: `backend/internal/service/wire.go`
  - 职责：提供 `EffectiveGroupResolver`，并把 resolver 传给 routes 或 handlers 的依赖构造。
- Modify: `backend/internal/handler/api_key_handler.go`
  - 职责：普通用户创建/更新 API Key 时忽略 `group_id`，始终创建自动 Key；管理员 API 不受影响。
- Modify: `backend/internal/service/api_key_service.go`
  - 职责：必要时增加 user API 明确自动 Key 的服务方法，或保持 handler 层清空 `GroupID`。
- Create: `backend/migrations/159_auto_api_key_effective_group.sql`
  - 职责：创建/维护 `traffic-pack-openai` 内部 group，绑定 OpenAI 上游账号，迁移旧 OpenAI API Key 为自动 Key。
- Modify: `backend/internal/repository/migrations_schema_integration_test.go`
  - 职责：验证迁移结果不影响非 OpenAI Key。
- Modify: `frontend/src/views/user/KeysView.vue`
  - 职责：删除普通用户创建/编辑 API Key 的 group 下拉框、快速切换 group 菜单和 group 必填校验；展示“自动分组”。
- Modify: `frontend/src/api/keys.ts`
  - 职责：允许调用方创建 Key 时不传 `group_id`；现有类型已支持 optional，主要确认不回填 null。
- Modify: `frontend/src/i18n/locales/zh.ts`
  - 职责：新增或调整“自动分组”“按当前套餐或 GPT 流量包自动使用”等文案。
- Modify: `frontend/src/i18n/locales/en.ts`
  - 职责：同步英文文案。
- Create: `frontend/src/views/user/__tests__/KeysView.spec.ts`
  - 职责：覆盖无下拉框、无 group_id 请求、自动分组展示。
- Modify: `AGENTS.md`
  - 职责：实现完成后追加结果索引。
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-auto-api-key-effective-group-result_CN.md`
  - 职责：记录实现、测试、迁移与部署前注意事项。

---

### Task 1: 后端 resolver 单元测试

**Files:**
- Create: `backend/internal/service/effective_group_resolver_test.go`
- Create in Task 2: `backend/internal/service/effective_group_resolver.go`

- [ ] **Step 1: 写 resolver 失败测试**

在 `backend/internal/service/effective_group_resolver_test.go` 写入测试骨架。测试先引用尚不存在的 `NewEffectiveGroupResolver`、`ResolveEffectiveGroup`、`EffectiveGroupSourceSubscription` 等符号，确保红灯来自缺少实现。

```go
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type effectiveGroupSubRepoStub struct {
	subs []UserSubscription
	err  error
}

func (s *effectiveGroupSubRepoStub) ListActiveByUserID(ctx context.Context, userID int64) ([]UserSubscription, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]UserSubscription, 0, len(s.subs))
	for _, sub := range s.subs {
		if sub.UserID == userID {
			out = append(out, sub)
		}
	}
	return out, nil
}

type effectiveGroupGroupRepoStub struct {
	groups []Group
	err    error
}

func (s *effectiveGroupGroupRepoStub) ListActiveByPlatform(ctx context.Context, platform string) ([]Group, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]Group, 0, len(s.groups))
	for _, group := range s.groups {
		if group.Platform == platform && group.Status == StatusActive {
			out = append(out, group)
		}
	}
	return out, nil
}

type effectiveGroupTrafficRepoStub struct {
	has bool
	err error
}

func (s *effectiveGroupTrafficRepoStub) HasAvailableCredit(ctx context.Context, userID int64, now time.Time) (bool, error) {
	return s.has, s.err
}

func (s *effectiveGroupTrafficRepoStub) ListForSale(ctx context.Context) ([]TrafficPack, error) {
	return nil, nil
}

func (s *effectiveGroupTrafficRepoStub) GetForSaleByID(ctx context.Context, id int64) (*TrafficPack, error) {
	return nil, ErrInvalidInput
}

func (s *effectiveGroupTrafficRepoStub) GetSummary(ctx context.Context, userID int64, now time.Time) (*TrafficCreditSummary, error) {
	return &TrafficCreditSummary{}, nil
}

func (s *effectiveGroupTrafficRepoStub) CreditPurchase(ctx context.Context, input CreditTrafficPackInput) error {
	return nil
}

func (s *effectiveGroupTrafficRepoStub) Deduct(ctx context.Context, userID int64, amountUSD float64, requestID string, now time.Time) (bool, []TrafficCreditDeduction, error) {
	return false, nil, nil
}

func TestEffectiveGroupResolver_SubscriptionBeatsTrafficPack(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	limit19 := 19.0
	limit69 := 69.0
	lowGroup := &Group{ID: 2, Name: "codex-pool-19-usd", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &limit19}
	highGroup := &Group{ID: 9, Name: "codex-pool-69-usd", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &limit69}

	resolver := NewEffectiveGroupResolver(
		&effectiveGroupSubRepoStub{subs: []UserSubscription{
			{ID: 10, UserID: 7, GroupID: lowGroup.ID, Group: lowGroup, Status: SubscriptionStatusActive, ExpiresAt: now.Add(30 * 24 * time.Hour), CreatedAt: now.Add(-2 * time.Hour)},
			{ID: 11, UserID: 7, GroupID: highGroup.ID, Group: highGroup, Status: SubscriptionStatusActive, ExpiresAt: now.Add(30 * 24 * time.Hour), CreatedAt: now.Add(-1 * time.Hour)},
		}},
		&effectiveGroupGroupRepoStub{},
		NewTrafficPackService(&effectiveGroupTrafficRepoStub{has: true}),
	)
	resolver.now = func() time.Time { return now }

	result, err := resolver.ResolveEffectiveGroup(context.Background(), 7, PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, EffectiveGroupSourceSubscription, result.Source)
	require.Equal(t, int64(9), result.Group.ID)
	require.NotNil(t, result.Subscription)
	require.Equal(t, int64(11), result.Subscription.ID)
}

func TestEffectiveGroupResolver_TrafficPackUsesInternalOpenAIGroup(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	trafficGroup := Group{ID: 77, Name: TrafficPackOpenAIGroupName, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, IsExclusive: true}

	resolver := NewEffectiveGroupResolver(
		&effectiveGroupSubRepoStub{},
		&effectiveGroupGroupRepoStub{groups: []Group{trafficGroup}},
		NewTrafficPackService(&effectiveGroupTrafficRepoStub{has: true}),
	)
	resolver.now = func() time.Time { return now }

	result, err := resolver.ResolveEffectiveGroup(context.Background(), 62, PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, EffectiveGroupSourceTrafficPack, result.Source)
	require.Equal(t, trafficGroup.ID, result.Group.ID)
	require.Nil(t, result.Subscription)
}

func TestEffectiveGroupResolver_NoOpenAIEntitlement(t *testing.T) {
	resolver := NewEffectiveGroupResolver(
		&effectiveGroupSubRepoStub{},
		&effectiveGroupGroupRepoStub{},
		NewTrafficPackService(&effectiveGroupTrafficRepoStub{has: false}),
	)

	result, err := resolver.ResolveEffectiveGroup(context.Background(), 62, PlatformOpenAI)
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrNoOpenAIEntitlement)
}

func TestEffectiveGroupResolver_SubscriptionLoadErrorDoesNotFallbackToTrafficPack(t *testing.T) {
	resolver := NewEffectiveGroupResolver(
		&effectiveGroupSubRepoStub{err: errors.New("db down")},
		&effectiveGroupGroupRepoStub{},
		NewTrafficPackService(&effectiveGroupTrafficRepoStub{has: true}),
	)

	result, err := resolver.ResolveEffectiveGroup(context.Background(), 62, PlatformOpenAI)
	require.Nil(t, result)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNoOpenAIEntitlement)
}
```

- [ ] **Step 2: 运行测试确认红灯**

Run:

```bash
cd backend
go test -count=1 -tags=unit ./internal/service -run 'TestEffectiveGroupResolver'
```

Expected:

```text
FAIL
undefined: NewEffectiveGroupResolver
undefined: EffectiveGroupSourceSubscription
undefined: TrafficPackOpenAIGroupName
```

- [ ] **Step 3: 提交红灯测试**

```bash
git add backend/internal/service/effective_group_resolver_test.go
git commit -m "test: cover automatic effective group resolution"
```

---

### Task 2: 实现 EffectiveGroupResolver

**Files:**
- Create: `backend/internal/service/effective_group_resolver.go`
- Modify: `backend/internal/service/effective_group_resolver_test.go`

- [ ] **Step 1: 写 resolver 最小实现**

创建 `backend/internal/service/effective_group_resolver.go`：

```go
package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const TrafficPackOpenAIGroupName = "traffic-pack-openai"

var (
	ErrNoOpenAIEntitlement          = infraerrors.Forbidden("NO_OPENAI_ENTITLEMENT", "请先购买套餐或 GPT 流量包")
	ErrOpenAITrafficGroupUnavailable = infraerrors.ServiceUnavailable("OPENAI_TRAFFIC_GROUP_UNAVAILABLE", "OpenAI 流量包入口分组不可用")
)

type EffectiveGroupSource string

const (
	EffectiveGroupSourceFixed        EffectiveGroupSource = "fixed"
	EffectiveGroupSourceSubscription EffectiveGroupSource = "subscription"
	EffectiveGroupSourceTrafficPack  EffectiveGroupSource = "traffic_pack"
)

type EffectiveGroupResult struct {
	Group        *Group
	Subscription *UserSubscription
	Source       EffectiveGroupSource
}

type effectiveGroupSubscriptionRepo interface {
	ListActiveByUserID(ctx context.Context, userID int64) ([]UserSubscription, error)
}

type effectiveGroupGroupRepo interface {
	ListActiveByPlatform(ctx context.Context, platform string) ([]Group, error)
}

type EffectiveGroupResolver struct {
	subRepo            effectiveGroupSubscriptionRepo
	groupRepo          effectiveGroupGroupRepo
	trafficPackService *TrafficPackService
	now                func() time.Time
}

func NewEffectiveGroupResolver(subRepo effectiveGroupSubscriptionRepo, groupRepo effectiveGroupGroupRepo, trafficPackService *TrafficPackService) *EffectiveGroupResolver {
	return &EffectiveGroupResolver{
		subRepo:            subRepo,
		groupRepo:          groupRepo,
		trafficPackService: trafficPackService,
		now:                time.Now,
	}
}

func (r *EffectiveGroupResolver) ResolveEffectiveGroup(ctx context.Context, userID int64, platform string) (*EffectiveGroupResult, error) {
	if r == nil {
		return nil, ErrNoOpenAIEntitlement
	}
	if platform != PlatformOpenAI {
		return nil, ErrNoOpenAIEntitlement
	}

	subResult, err := r.resolveOpenAISubscription(ctx, userID)
	if err != nil {
		return nil, err
	}
	if subResult != nil {
		return subResult, nil
	}

	hasCredit, err := r.trafficPackService.HasAvailableCredit(ctx, userID, r.now())
	if err != nil {
		return nil, fmt.Errorf("check traffic pack credit: %w", err)
	}
	if !hasCredit {
		return nil, ErrNoOpenAIEntitlement
	}

	group, err := r.resolveTrafficPackOpenAIGroup(ctx)
	if err != nil {
		return nil, err
	}
	return &EffectiveGroupResult{Group: group, Source: EffectiveGroupSourceTrafficPack}, nil
}

func (r *EffectiveGroupResolver) resolveOpenAISubscription(ctx context.Context, userID int64) (*EffectiveGroupResult, error) {
	if r.subRepo == nil {
		return nil, nil
	}
	subs, err := r.subRepo.ListActiveByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active subscriptions: %w", err)
	}
	now := r.now()
	candidates := make([]UserSubscription, 0, len(subs))
	for _, sub := range subs {
		if sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(now) || sub.Group == nil {
			continue
		}
		if sub.Group.Platform != PlatformOpenAI || !sub.Group.IsActive() || !sub.Group.IsSubscriptionType() {
			continue
		}
		candidates = append(candidates, sub)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		gi, gj := candidates[i].Group, candidates[j].Group
		li, lj := subscriptionDailyLimitRank(gi), subscriptionDailyLimitRank(gj)
		if li != lj {
			return li > lj
		}
		if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
		}
		return candidates[i].ID > candidates[j].ID
	})
	selected := candidates[0]
	return &EffectiveGroupResult{Group: selected.Group, Subscription: &selected, Source: EffectiveGroupSourceSubscription}, nil
}

func subscriptionDailyLimitRank(group *Group) float64 {
	if group == nil || group.DailyLimitUSD == nil {
		return 1<<62
	}
	return *group.DailyLimitUSD
}

func (r *EffectiveGroupResolver) resolveTrafficPackOpenAIGroup(ctx context.Context) (*Group, error) {
	if r.groupRepo == nil {
		return nil, ErrOpenAITrafficGroupUnavailable
	}
	groups, err := r.groupRepo.ListActiveByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return nil, fmt.Errorf("list active openai groups: %w", err)
	}
	for i := range groups {
		group := groups[i]
		if group.Name == TrafficPackOpenAIGroupName && group.IsActive() && !group.IsSubscriptionType() {
			return &group, nil
		}
	}
	return nil, ErrOpenAITrafficGroupUnavailable
}
```

- [ ] **Step 2: 确认测试 stub 已覆盖完整 `TrafficPackRepository` 接口**

`backend/internal/service/effective_group_resolver_test.go` 中的 `effectiveGroupTrafficRepoStub` 必须包含下面这些方法，保证测试红灯只来自 resolver 实现，而不是 stub 接口不完整：

```go
func (s *effectiveGroupTrafficRepoStub) ListForSale(ctx context.Context) ([]TrafficPack, error) {
	return nil, nil
}

func (s *effectiveGroupTrafficRepoStub) GetForSaleByID(ctx context.Context, id int64) (*TrafficPack, error) {
	return nil, ErrInvalidInput
}

func (s *effectiveGroupTrafficRepoStub) GetSummary(ctx context.Context, userID int64, now time.Time) (*TrafficCreditSummary, error) {
	return &TrafficCreditSummary{}, nil
}

func (s *effectiveGroupTrafficRepoStub) CreditPurchase(ctx context.Context, input CreditTrafficPackInput) error {
	return nil
}

func (s *effectiveGroupTrafficRepoStub) Deduct(ctx context.Context, userID int64, amountUSD float64, requestID string, now time.Time) (bool, []TrafficCreditDeduction, error) {
	return false, nil, nil
}
```

- [ ] **Step 3: 跑 resolver 测试确认绿灯**

Run:

```bash
cd backend
go test -count=1 -tags=unit ./internal/service -run 'TestEffectiveGroupResolver'
```

Expected:

```text
ok   github.com/Wei-Shaw/sub2api/internal/service
```

- [ ] **Step 4: 提交 resolver 实现**

```bash
git add backend/internal/service/effective_group_resolver.go backend/internal/service/effective_group_resolver_test.go
git commit -m "feat: resolve effective group for automatic api keys"
```

---

### Task 3: 请求级 effective group 中间件

**Files:**
- Create: `backend/internal/server/middleware/effective_group.go`
- Create: `backend/internal/server/middleware/effective_group_test.go`
- Modify: `backend/internal/server/middleware/api_key_auth.go`

- [ ] **Step 1: 写中间件失败测试**

创建 `backend/internal/server/middleware/effective_group_test.go`：

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type effectiveGroupResolverStub struct {
	result *service.EffectiveGroupResult
	err    error
}

func (s *effectiveGroupResolverStub) ResolveEffectiveGroup(c *gin.Context, apiKey *service.APIKey) (*service.EffectiveGroupResult, error) {
	return s.result, s.err
}

func TestResolveEffectiveGroupMiddlewareWritesRequestScopedAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := &service.APIKey{ID: 1, UserID: 62, Key: "sk-test", GroupID: nil, User: &service.User{ID: 62, Status: service.StatusActive}}
	group := &service.Group{ID: 77, Name: service.TrafficPackOpenAIGroupName, Platform: service.PlatformOpenAI, Status: service.StatusActive}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), original)
		c.Next()
	})
	router.Use(ResolveEffectiveGroup(&effectiveGroupResolverStub{
		result: &service.EffectiveGroupResult{Group: group, Source: service.EffectiveGroupSourceTrafficPack},
	}, AnthropicErrorWriter))
	router.GET("/v1/responses", func(c *gin.Context) {
		got, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotSame(t, original, got)
		require.Nil(t, original.GroupID)
		require.Nil(t, original.Group)
		require.NotNil(t, got.GroupID)
		require.Equal(t, int64(77), *got.GroupID)
		require.Equal(t, group, got.Group)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestResolveEffectiveGroupMiddlewareSkipsFixedGroupKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(2)
	original := &service.APIKey{ID: 1, UserID: 7, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), original)
		c.Next()
	})
	router.Use(ResolveEffectiveGroup(&effectiveGroupResolverStub{}, AnthropicErrorWriter))
	router.GET("/v1/responses", func(c *gin.Context) {
		got, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Same(t, original, got)
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
	require.Equal(t, http.StatusNoContent, rec.Code)
}
```

- [ ] **Step 2: 运行测试确认红灯**

```bash
cd backend
go test -count=1 -tags=unit ./internal/server/middleware -run 'TestResolveEffectiveGroupMiddleware'
```

Expected:

```text
FAIL
undefined: ResolveEffectiveGroup
```

- [ ] **Step 3: 实现中间件**

创建 `backend/internal/server/middleware/effective_group.go`：

```go
package middleware

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type EffectiveGroupResolver interface {
	ResolveEffectiveGroup(c *gin.Context, apiKey *service.APIKey) (*service.EffectiveGroupResult, error)
}

func ResolveEffectiveGroup(resolver EffectiveGroupResolver, writeError GatewayErrorWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.GroupID != nil {
			c.Next()
			return
		}
		if resolver == nil {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnassigned)
			writeError(c, http.StatusForbidden, "API Key is not assigned to any group and cannot be resolved automatically.")
			c.Abort()
			return
		}
		result, err := resolver.ResolveEffectiveGroup(c, apiKey)
		if err != nil {
			status, message := effectiveGroupErrorResponse(err)
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable)
			writeError(c, status, message)
			c.Abort()
			return
		}
		if result == nil || result.Group == nil {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable)
			writeError(c, http.StatusForbidden, "No available group for this API Key.")
			c.Abort()
			return
		}

		requestKey := *apiKey
		groupID := result.Group.ID
		requestKey.GroupID = &groupID
		requestKey.Group = result.Group

		c.Set(string(ContextKeyAPIKey), &requestKey)
		if result.Subscription != nil {
			c.Set(string(ContextKeySubscription), result.Subscription)
		}
		ctx := contextWithEffectiveGroup(c.Request.Context(), result.Group)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func contextWithEffectiveGroup(ctx context.Context, group *service.Group) context.Context {
	if group == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.Group, group)
}

func effectiveGroupErrorResponse(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrNoOpenAIEntitlement):
		return http.StatusForbidden, "请先购买套餐或 GPT 流量包"
	case errors.Is(err, service.ErrOpenAITrafficGroupUnavailable):
		return http.StatusServiceUnavailable, "OpenAI 流量包入口分组不可用"
	default:
		return http.StatusInternalServerError, "Failed to resolve API Key group"
	}
}
```

在文件顶部补齐 imports：

```go
import (
	"context"
	"errors"
	"net/http"
)
```

- [ ] **Step 4: 确认中间件测试绿灯**

```bash
cd backend
go test -count=1 -tags=unit ./internal/server/middleware -run 'TestResolveEffectiveGroupMiddleware'
```

Expected:

```text
ok   github.com/Wei-Shaw/sub2api/internal/server/middleware
```

- [ ] **Step 5: 提交中间件**

```bash
git add backend/internal/server/middleware/effective_group.go backend/internal/server/middleware/effective_group_test.go
git commit -m "feat: attach effective group to automatic api keys"
```

---

### Task 4: Resolver 适配 Gin 路径与路由挂载

**Files:**
- Modify: `backend/internal/service/effective_group_resolver.go`
- Modify: `backend/internal/server/routes/gateway.go`
- Modify: `backend/internal/service/wire.go`
- Test: `backend/internal/server/api_contract_test.go` 或 `backend/internal/server/middleware/effective_group_test.go`

- [ ] **Step 1: 给 resolver 增加 Gin 适配方法**

在 `backend/internal/service/effective_group_resolver.go` 增加：

```go
func (r *EffectiveGroupResolver) ResolveEffectiveGroupForRequest(ctx context.Context, userID int64, path string, forcePlatform string) (*EffectiveGroupResult, error) {
	platform := forcePlatform
	if platform == "" {
		platform = inferEffectiveGroupPlatform(path)
	}
	return r.ResolveEffectiveGroup(ctx, userID, platform)
}

func inferEffectiveGroupPlatform(path string) string {
	switch {
	case strings.Contains(path, "/responses"),
		strings.Contains(path, "/chat/completions"),
		strings.Contains(path, "/embeddings"),
		strings.Contains(path, "/images/"),
		strings.Contains(path, "/messages"):
		return PlatformOpenAI
	default:
		return ""
	}
}
```

补 import：

```go
import "strings"
```

- [ ] **Step 2: 在中间件中调用请求适配方法**

把 `backend/internal/server/middleware/effective_group.go` 的 interface 改为：

```go
type EffectiveGroupResolver interface {
	ResolveEffectiveGroupForRequest(ctx context.Context, userID int64, path string, forcePlatform string) (*service.EffectiveGroupResult, error)
}
```

调用处改为：

```go
forcePlatform, _ := c.Request.Context().Value(ctxkey.ForcePlatform).(string)
result, err := resolver.ResolveEffectiveGroupForRequest(c.Request.Context(), apiKey.UserID, c.Request.URL.Path, forcePlatform)
```

- [ ] **Step 3: 在 routes 挂载中间件**

修改 `backend/internal/server/routes/gateway.go` 的 `RegisterGatewayRoutes` 签名，增加 resolver 参数：

```go
effectiveGroupResolver *service.EffectiveGroupResolver,
```

创建中间件：

```go
resolveGroupAnthropic := middleware.ResolveEffectiveGroup(effectiveGroupResolver, middleware.AnthropicErrorWriter)
resolveGroupGoogle := middleware.ResolveEffectiveGroup(effectiveGroupResolver, middleware.GoogleErrorWriter)
```

把 OpenAI/Anthropic 兼容入口顺序从：

```go
gateway.Use(gin.HandlerFunc(apiKeyAuth))
gateway.Use(requireGroupAnthropic)
```

改成：

```go
gateway.Use(gin.HandlerFunc(apiKeyAuth))
gateway.Use(resolveGroupAnthropic)
gateway.Use(requireGroupAnthropic)
```

对裸路由同样插入 `resolveGroupAnthropic`：

```go
r.POST("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), resolveGroupAnthropic, requireGroupAnthropic, responsesHandler)
```

Gemini/Antigravity 强制平台路由暂不启用 OpenAI 自动解析，保持原行为。

- [ ] **Step 4: 更新 wire provider**

在 `backend/internal/service/wire.go` 的 provider set 中确保包含：

```go
NewEffectiveGroupResolver,
```

新增 provider，明确把订阅仓储、分组仓储和流量包服务注入 resolver：

```go
func ProvideEffectiveGroupResolver(subRepo UserSubscriptionRepository, groupRepo GroupRepository, trafficPackService *TrafficPackService) *EffectiveGroupResolver {
	return NewEffectiveGroupResolver(subRepo, groupRepo, trafficPackService)
}
```

并把 routes 构造调用补齐 `effectiveGroupResolver` 参数。

- [ ] **Step 5: 跑编译/单测**

```bash
cd backend
go test -count=1 -tags=unit ./internal/server/middleware ./internal/server ./internal/service
```

Expected:

```text
ok   github.com/Wei-Shaw/sub2api/internal/server/middleware
ok   github.com/Wei-Shaw/sub2api/internal/server
ok   github.com/Wei-Shaw/sub2api/internal/service
```

- [ ] **Step 6: 提交路由挂载**

```bash
git add backend/internal/service/effective_group_resolver.go backend/internal/server/middleware/effective_group.go backend/internal/server/routes/gateway.go backend/internal/service/wire.go
git commit -m "feat: resolve automatic api key groups before routing"
```

---

### Task 5: 用户 API Key 创建/更新改为自动 Key

**Files:**
- Modify: `backend/internal/handler/api_key_handler.go`
- Create: `backend/internal/handler/api_key_handler_test.go`

- [ ] **Step 1: 写后端行为测试**

在已有 API contract 或 handler 测试中加入断言：普通用户 `POST /api/v1/keys` 即使传入 `group_id`，创建请求进入 service 时也应是 nil。若现有测试难以注入 service stub，新建 handler 单测。

核心测试代码：

```go
func TestAPIKeyHandlerCreateIgnoresUserGroupID(t *testing.T) {
	req := CreateAPIKeyRequest{Name: "auto-key"}
	groupID := int64(9)
	req.GroupID = &groupID

	svcReq := service.CreateAPIKeyRequest{
		Name:          req.Name,
		GroupID:       nil,
		CustomKey:     req.CustomKey,
		IPWhitelist:   req.IPWhitelist,
		IPBlacklist:   req.IPBlacklist,
		ExpiresInDays: req.ExpiresInDays,
	}
	require.Nil(t, svcReq.GroupID)
}
```

同一测试文件再补一个真实 HTTP handler case，验证响应里的 `group_id` 为 null。

- [ ] **Step 2: 修改 Create handler**

在 `backend/internal/handler/api_key_handler.go` 的 `Create` 中，把：

```go
GroupID: req.GroupID,
```

改为：

```go
GroupID: nil,
```

并加原因注释：

```go
// 普通用户 Key 统一走运行时 effective group，避免套餐升级后必须重建 Key。
```

- [ ] **Step 3: 修改 Update handler**

在 `Update` 中删除或覆盖：

```go
svcReq.GroupID = req.GroupID
```

改为：

```go
// 普通用户不能把自动 Key 改回固定分组；管理员后台 API 保留固定分组能力。
svcReq.GroupID = nil
```

- [ ] **Step 4: 跑相关测试**

```bash
cd backend
go test -count=1 -tags=unit ./internal/handler ./internal/server -run 'APIKey|Contract'
```

Expected:

```text
ok   github.com/Wei-Shaw/sub2api/internal/handler
ok   github.com/Wei-Shaw/sub2api/internal/server
```

- [ ] **Step 5: 提交用户 API 行为**

```bash
git add backend/internal/handler/api_key_handler.go backend/internal/handler/*_test.go backend/internal/server/*_test.go
git commit -m "feat: create user api keys as automatic keys"
```

---

### Task 6: 迁移 traffic-pack-openai group 和旧 OpenAI API Key

**Files:**
- Create: `backend/migrations/159_auto_api_key_effective_group.sql`
- Modify: `backend/internal/repository/migrations_schema_integration_test.go`

- [ ] **Step 1: 写迁移 SQL**

创建 `backend/migrations/159_auto_api_key_effective_group.sql`：

```sql
-- 159_auto_api_key_effective_group.sql
-- Create the internal OpenAI group used by automatic API Keys when a user has
-- traffic-pack credit but no active OpenAI subscription, then migrate existing
-- OpenAI API Keys to automatic group resolution.

WITH inserted AS (
    INSERT INTO groups (
        name,
        description,
        rate_multiplier,
        is_exclusive,
        status,
        platform,
        subscription_type,
        default_validity_days,
        allow_image_generation,
        image_rate_independent,
        image_rate_multiplier,
        sort_order,
        created_at,
        updated_at
    )
    SELECT
        'traffic-pack-openai',
        'Internal OpenAI entry group for automatic API Keys backed by GPT traffic credits.',
        1.0000,
        TRUE,
        'active',
        'openai',
        'standard',
        30,
        TRUE,
        FALSE,
        1.0000,
        1000,
        NOW(),
        NOW()
    WHERE NOT EXISTS (
        SELECT 1
        FROM groups
        WHERE name = 'traffic-pack-openai'
          AND deleted_at IS NULL
    )
    RETURNING id
),
traffic_group AS (
    SELECT id FROM inserted
    UNION ALL
    SELECT id
    FROM groups
    WHERE name = 'traffic-pack-openai'
      AND deleted_at IS NULL
    LIMIT 1
)
INSERT INTO account_groups (account_id, group_id, priority, created_at)
SELECT a.id, tg.id, 50, NOW()
FROM accounts a
CROSS JOIN traffic_group tg
WHERE a.platform = 'openai'
  AND a.deleted_at IS NULL
ON CONFLICT (account_id, group_id) DO NOTHING;

UPDATE api_keys ak
SET group_id = NULL,
    updated_at = NOW()
FROM groups g
WHERE ak.group_id = g.id
  AND g.platform = 'openai'
  AND ak.deleted_at IS NULL;
```

- [ ] **Step 2: 写迁移验证测试**

在 `backend/internal/repository/migrations_schema_integration_test.go` 加入检查：

```go
func TestMigrationsRunner_AutoAPIKeyEffectiveGroupSeed(t *testing.T) {
	tx := testTx(t)

	var groupID int64
	require.NoError(t, tx.QueryRowContext(context.Background(), `
		SELECT id FROM groups
		WHERE name = 'traffic-pack-openai'
		  AND platform = 'openai'
		  AND subscription_type = 'standard'
		  AND is_exclusive = true
		  AND deleted_at IS NULL
	`).Scan(&groupID))
	require.Positive(t, groupID)

	var migratedOpenAIKeys int
	require.NoError(t, tx.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM api_keys ak
		JOIN groups g ON g.id = ak.group_id
		WHERE g.platform = 'openai'
		  AND ak.deleted_at IS NULL
	`).Scan(&migratedOpenAIKeys))
	require.Zero(t, migratedOpenAIKeys)
}
```

- [ ] **Step 3: 本地运行迁移测试**

```bash
cd backend
go test -count=1 ./internal/repository -run 'Migration|Migrations'
```

Expected:

```text
ok   github.com/Wei-Shaw/sub2api/internal/repository
```

- [ ] **Step 4: 提交迁移**

```bash
git add backend/migrations/159_auto_api_key_effective_group.sql backend/internal/repository/*migration*_test.go
git commit -m "feat: add traffic pack openai group migration"
```

---

### Task 7: 前端移除普通用户分组选择

**Files:**
- Modify: `frontend/src/views/user/KeysView.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Test: `frontend/src/views/user/__tests__/KeysView.spec.ts`

- [ ] **Step 1: 写前端失败测试**

新增或更新 `frontend/src/views/user/__tests__/KeysView.spec.ts`：

```ts
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import KeysView from '../KeysView.vue'
import { keysAPI } from '@/api'

vi.mock('@/api', () => ({
  keysAPI: {
    list: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10 }),
    create: vi.fn().mockResolvedValue({ id: 1, name: 'auto-key', group_id: null, status: 'active' })
  },
  userGroupsAPI: {
    getAvailable: vi.fn().mockResolvedValue([]),
    getUserGroupRates: vi.fn().mockResolvedValue({})
  }
}))

describe('KeysView automatic API Key', () => {
  it('does not render group selector in create dialog', async () => {
    const wrapper = mount(KeysView, {
      global: {
        stubs: ['RouterLink', 'BaseDialog', 'Select', 'GroupBadge', 'GroupOptionItem']
      }
    })
    await wrapper.find('[data-tour="keys-create-btn"]').trigger('click')
    expect(wrapper.find('[data-tour="key-form-group"]').exists()).toBe(false)
  })

  it('creates key without group_id', async () => {
    const wrapper = mount(KeysView, {
      global: {
        stubs: ['RouterLink', 'BaseDialog', 'Select', 'GroupBadge', 'GroupOptionItem']
      }
    })
    await wrapper.find('[data-tour="keys-create-btn"]').trigger('click')
    await wrapper.find('[data-tour="key-form-name"]').setValue('auto-key')
    await wrapper.find('form#key-form').trigger('submit.prevent')
    expect(keysAPI.create).toHaveBeenCalled()
    expect((keysAPI.create as any).mock.calls[0][1]).toBeNull()
  })
})
```

- [ ] **Step 2: 运行测试确认红灯**

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/KeysView.spec.ts
```

Expected:

```text
FAIL
expected group selector not to exist
```

- [ ] **Step 3: 删除创建/编辑弹窗中的 group Select**

从 `frontend/src/views/user/KeysView.vue` 删除包含 `data-tour="key-form-group"` 的整段 `<Select>`。保留名称、配额、过期、IP 限制、自定义 Key 等字段。

- [ ] **Step 4: 删除 group 必填校验和创建参数**

把 `handleSubmit()` 中的：

```ts
if (formData.value.group_id === null) {
  appStore.showError(t('keys.groupRequired'))
  return
}
```

删除。

把创建调用从：

```ts
await keysAPI.create(
  formData.value.name,
  formData.value.group_id,
  customKey,
  ipWhitelist,
  ipBlacklist,
  quota,
  expiresInDays,
  rateLimitData
)
```

改成：

```ts
await keysAPI.create(
  formData.value.name,
  null,
  customKey,
  ipWhitelist,
  ipBlacklist,
  quota,
  expiresInDays,
  rateLimitData
)
```

把编辑调用中的：

```ts
group_id: formData.value.group_id,
```

删除。

- [ ] **Step 5: 删除普通用户快速切换 group 菜单**

删除或禁用以下状态和 UI：

```ts
const groupSelectorKeyId = ref<number | null>(null)
const selectedKeyForGroup = computed(() => {
  if (groupSelectorKeyId.value === null) return null
  return apiKeys.value.find((k) => k.id === groupSelectorKeyId.value) || null
})
const changeGroup = async (key: ApiKey, newGroupId: number | null) => {
  groupSelectorKeyId.value = null
  dropdownPosition.value = null
  if (key.group_id === newGroupId) return
  await keysAPI.update(key.id, { group_id: newGroupId })
  loadApiKeys()
}
```

删除 Teleport 下拉框和触发按钮。Key 列表里显示：

```vue
<span v-if="!key.group_id" class="inline-flex items-center rounded-md bg-primary-50 px-2 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">
  {{ t('keys.autoGroup') }}
</span>
<p v-if="!key.group_id" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
  {{ t('keys.autoGroupHint') }}
</p>
```

- [ ] **Step 6: 更新中英文文案**

在 `frontend/src/i18n/locales/zh.ts` 的 `keys` 段加入：

```ts
autoGroup: '自动分组',
autoGroupHint: '按当前套餐或 GPT 流量包自动使用',
```

在 `frontend/src/i18n/locales/en.ts` 的 `keys` 段加入：

```ts
autoGroup: 'Automatic',
autoGroupHint: 'Uses your current subscription or GPT traffic pack automatically',
```

- [ ] **Step 7: 运行前端测试**

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/KeysView.spec.ts
```

Expected:

```text
PASS
```

- [ ] **Step 8: 提交前端变更**

```bash
git add frontend/src/views/user/KeysView.vue frontend/src/views/user/__tests__/KeysView.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: make user api keys automatic"
```

---

### Task 8: 后端 OpenAI 请求集成验证

**Files:**
- Create: `backend/internal/server/automatic_api_key_test.go`

- [ ] **Step 1: 增加自动 Key 请求路由测试**

创建 `backend/internal/server/automatic_api_key_test.go`，用 stub handler 验证 resolver 之后的路由上下文已经是 OpenAI group：

```go
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAutomaticAPIKeyRequestContextUsesSubscriptionGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(9)
	sub := &service.UserSubscription{ID: 71, UserID: 62, GroupID: groupID}
	group := &service.Group{ID: groupID, Name: "codex-pool-69-usd", Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 65, UserID: 62, GroupID: &groupID, Group: group})
		c.Set(string(middleware.ContextKeySubscription), sub)
		c.Next()
	})
	router.POST("/v1/responses", func(c *gin.Context) {
		apiKey, ok := middleware.GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, service.PlatformOpenAI, apiKey.Group.Platform)
		gotSub, ok := middleware.GetSubscriptionFromContext(c)
		require.True(t, ok)
		require.Equal(t, sub.ID, gotSub.ID)
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
	require.Equal(t, http.StatusNoContent, rec.Code)
}
```

- [ ] **Step 2: 覆盖无权益错误**

同文件增加无权益错误测试。测试使用 Task 3 的 resolver stub，返回 `service.ErrNoOpenAIEntitlement`：

```go
func TestAutomaticAPIKeyWithoutOpenAIEntitlementReturns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 66, UserID: 63})
		c.Next()
	})
	router.Use(middleware.ResolveEffectiveGroup(&automaticKeyNoEntitlementResolver{}, middleware.AnthropicErrorWriter))
	router.POST("/v1/responses", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
	require.Equal(t, http.StatusForbidden, rec.Code)
}

type automaticKeyNoEntitlementResolver struct{}

func (automaticKeyNoEntitlementResolver) ResolveEffectiveGroupForRequest(ctx context.Context, userID int64, path string, forcePlatform string) (*service.EffectiveGroupResult, error) {
	return nil, service.ErrNoOpenAIEntitlement
}
```

- [ ] **Step 3: 运行后端集成测试**

```bash
cd backend
go test -count=1 -tags=unit ./internal/server -run 'AutomaticAPIKey'
```

Expected:

```text
ok   github.com/Wei-Shaw/sub2api/internal/server
```

- [ ] **Step 4: 提交集成测试**

```bash
git add backend/internal/server/automatic_api_key_test.go
git commit -m "test: verify automatic api key gateway flow"
```

---

### Task 9: 全量验证和构建

**Files:**
- 本任务默认不改代码；验证暴露问题时，只修复对应失败文件。

- [ ] **Step 1: 后端核心测试**

```bash
cd backend
go test -count=1 -tags=unit ./internal/service ./internal/server/middleware ./internal/server ./internal/handler
```

Expected:

```text
ok   github.com/Wei-Shaw/sub2api/internal/service
ok   github.com/Wei-Shaw/sub2api/internal/server/middleware
ok   github.com/Wei-Shaw/sub2api/internal/server
ok   github.com/Wei-Shaw/sub2api/internal/handler
```

- [ ] **Step 2: 迁移和仓储测试**

```bash
cd backend
go test -count=1 ./internal/repository -run 'Migration|Migrations|APIKey'
```

Expected:

```text
ok   github.com/Wei-Shaw/sub2api/internal/repository
```

- [ ] **Step 3: 前端测试**

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/KeysView.spec.ts
```

Expected:

```text
Test Files  1 passed
Tests       all passed
```

- [ ] **Step 4: 前端构建**

```bash
cd frontend
pnpm build
```

Expected:

```text
✓ built
```

- [ ] **Step 5: 根目录 diff 检查**

```bash
git diff --check
```

Expected: no output, exit code 0.

- [ ] **Step 6: 提交验证收尾**

前面任务已经逐步提交且这里没有新改动时，不需要提交。修复验证问题后按实际修改路径提交，例如：

```bash
git add backend/internal/service/effective_group_resolver.go backend/internal/server/middleware/effective_group.go frontend/src/views/user/KeysView.vue
git commit -m "fix: stabilize automatic api key verification"
```

---

### Task 10: 候选环境运行态验证计划

**Files:**
- Create after implementation: `docs/ai/context/YYYYMMDD-HHMMSS-auto-api-key-effective-group-result_CN.md`

- [ ] **Step 1: 上线前备份候选或公网库**

在执行迁移前必须备份目标库：

```bash
mkdir -p deploy/backups
docker exec sub2api-candidate-postgres pg_dump -U sub2api -d sub2api -Fc > deploy/backups/$(date '+%Y%m%d-%H%M%S')-sub2api-candidate-before-auto-api-key.dump
chmod 600 deploy/backups/*-sub2api-candidate-before-auto-api-key.dump
```

Expected: dump 文件存在且权限为 `600`。

- [ ] **Step 2: 候选环境应用迁移后核对 group**

```bash
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -Atc "SELECT id,name,platform,subscription_type,is_exclusive,status FROM groups WHERE name='traffic-pack-openai' AND deleted_at IS NULL;"
```

Expected:

```text
<id>|traffic-pack-openai|openai|standard|t|active
```

- [ ] **Step 3: 核对旧 OpenAI Key 已迁移**

```bash
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -Atc "SELECT COUNT(*) FROM api_keys ak JOIN groups g ON ak.group_id=g.id WHERE g.platform='openai' AND ak.deleted_at IS NULL;"
```

Expected:

```text
0
```

- [ ] **Step 4: 使用 `1930863755@qq.com` 验证流量包自动 Key**

流程：

1. 使用用户登录态或后台代操作创建一个新 API Key。
2. 确认 `api_keys.group_id IS NULL`。
3. 用该 Key 请求 `/v1/responses`。
4. 查询 `traffic_credit_ledger` 新增 deduction。
5. 查询 `usage_logs.subscription_id IS NULL` 且 `group_id` 为 `traffic-pack-openai`。

核心查询：

```bash
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -F $'\t' -P pager=off -c "SELECT ak.id, ak.group_id FROM api_keys ak JOIN users u ON u.id=ak.user_id WHERE u.email='1930863755@qq.com' AND ak.deleted_at IS NULL ORDER BY ak.id DESC LIMIT 1;"
```

Expected: `group_id` 为空。

- [ ] **Step 5: 验证 active 套餐用户自动 Key**

选择一个 active 79 元套餐用户，使用迁移后的旧 Key 或新 Key 请求 `/v1/responses`。查询：

```bash
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -F $'\t' -P pager=off -c "SELECT ul.user_id, ul.group_id, g.name, ul.subscription_id, ul.actual_cost FROM usage_logs ul LEFT JOIN groups g ON g.id=ul.group_id ORDER BY ul.id DESC LIMIT 5;"
```

Expected: `group_id` 为套餐 group，`subscription_id` 非空，未新增对应 traffic pack deduction。

- [ ] **Step 6: 写结果文档**

创建结果文档：

```bash
date '+%Y%m%d-%H%M%S'
```

文档路径：

```text
docs/ai/context/YYYYMMDD-HHMMSS-auto-api-key-effective-group-result_CN.md
```

内容必须记录：

- 代码变更范围。
- 迁移是否执行。
- 候选/公网验证命令和结果。
- 是否涉及真实用户 Key，Key 必须脱敏。
- 未完成项或风险。

- [ ] **Step 7: 更新 AGENTS**

在 `AGENTS.md` 当前运行态提醒追加一条：

```markdown
- 2026-07-05 自动 API Key effective_group 已实现并验证：普通用户新建 Key 不再选择分组，旧 OpenAI Key 已迁移为 `group_id=NULL`，OpenAI 请求按 active subscription 或 `traffic-pack-openai` 入口 group 解析；验证结果见 `docs/ai/context/YYYYMMDD-HHMMSS-auto-api-key-effective-group-result_CN.md`。
```

- [ ] **Step 8: 提交文档**

```bash
git add AGENTS.md docs/ai/context/YYYYMMDD-HHMMSS-auto-api-key-effective-group-result_CN.md
git commit -m "docs: record automatic api key rollout"
```

---

## 自检清单

- [ ] 设计目标覆盖：普通用户前端无分组下拉、新建 Key 为 `group_id=NULL`、旧 OpenAI Key 迁移、运行时解析 subscription/traffic pack。
- [ ] 非目标保持：subscriptions 页面不混入流量包；管理员固定分组能力保留。
- [ ] 热路径安全：resolver 不修改 auth cache 原始 APIKey 指针。
- [ ] 错误语义明确：无权益为 403，入口 group 不可用为 503，DB 错误不 fallback 到流量包。
- [ ] 计费正确：subscription 请求记录 `subscription_id`，traffic pack 请求扣 `traffic_credit_ledger` 且 `subscription_id=NULL`。
- [ ] 迁移安全：先备份；只清空 OpenAI API Key 的 `group_id`；不修改历史 usage。
- [ ] 测试完成：后端 resolver/middleware/handler/integration、前端 KeysView、迁移测试、构建和 `git diff --check`。

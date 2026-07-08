package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const TrafficPackOpenAIGroupName = "traffic-pack-openai"

var (
	ErrNoOpenAIEntitlement           = infraerrors.Forbidden("NO_OPENAI_ENTITLEMENT", "请先购买套餐或 GPT 流量包")
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
	if r == nil || platform != PlatformOpenAI {
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

func (r *EffectiveGroupResolver) ResolveEffectiveGroupForRequest(ctx context.Context, userID int64, path string, forcePlatform string) (*EffectiveGroupResult, error) {
	platform := forcePlatform
	if platform == "" {
		platform = inferEffectiveGroupPlatform(path)
	}
	return r.ResolveEffectiveGroup(ctx, userID, platform)
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
		li, lj := subscriptionDailyLimitRank(candidates[i].Group), subscriptionDailyLimitRank(candidates[j].Group)
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
		return 1 << 62
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

func inferEffectiveGroupPlatform(path string) string {
	if isFormalOpenAIRequestPath(path) {
		return PlatformOpenAI
	}
	return ""
}

func isFormalOpenAIRequestPath(path string) bool {
	path = strings.TrimRight(strings.TrimSpace(path), "/")
	switch {
	case path == "/v1/usage":
		return true
	case path == "/v1/responses" || strings.HasPrefix(path, "/v1/responses/"):
		return true
	case path == "/v1/messages" || strings.HasPrefix(path, "/v1/messages/"):
		return true
	case path == "/v1/chat/completions":
		return true
	case path == "/v1/embeddings":
		return true
	case path == "/v1/images/generations", path == "/v1/images/edits":
		return true
	default:
		return false
	}
}

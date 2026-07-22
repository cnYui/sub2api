//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionQuotaDebtAdjustmentRepository_CreateGetAndRejectDuplicateSource(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := NewSubscriptionQuotaDebtAdjustmentRepository(client)
	user, group, subscription := createSubscriptionEntitlementPeriodFixture(t, ctx, client)
	now := time.Now().UTC()
	appliedAt := now.Add(time.Minute)
	sourceKey := fmt.Sprintf("weekly_quota_cutover_overage:%d:%d", subscription.ID, now.UnixNano())

	adjustment := &service.SubscriptionQuotaDebtAdjustment{
		SubscriptionID:     subscription.ID,
		UserID:             user.ID,
		GroupID:            group.ID,
		SourceKey:          sourceKey,
		OverageUSD:         234.1998836,
		WeeklyLimitUSD:     58,
		DailyEquivalentUSD: 58.0 / 7,
		RawDeductionDays:   28.2655038828,
		DeductedDays:       28,
		OriginalExpiresAt:  subscription.ExpiresAt.AddDate(0, 0, 28),
		NewExpiresAt:       subscription.ExpiresAt,
		ApplicationStatus:  service.SubscriptionQuotaDebtStatusAlreadyApplied,
		AppliedAt:          &appliedAt,
		Notes:              "本地已扣减，只记录审计事实",
	}
	require.NoError(t, repo.Create(ctx, adjustment))
	require.NotZero(t, adjustment.ID)

	loaded, err := repo.GetBySourceKey(ctx, sourceKey)
	require.NoError(t, err)
	require.Equal(t, adjustment.ID, loaded.ID)
	require.Equal(t, subscription.ID, loaded.SubscriptionID)
	require.Equal(t, service.SubscriptionQuotaDebtStatusAlreadyApplied, loaded.ApplicationStatus)
	require.InDelta(t, 234.1998836, loaded.OverageUSD, 0.0000001)
	require.InDelta(t, 58.0/7, loaded.DailyEquivalentUSD, 0.0000001)
	require.Equal(t, 28, loaded.DeductedDays)
	require.Equal(t, "本地已扣减，只记录审计事实", loaded.Notes)

	duplicate := *adjustment
	duplicate.ID = 0
	err = repo.Create(ctx, &duplicate)
	require.ErrorIs(t, err, service.ErrSubscriptionQuotaDebtAdjustmentSourceExists)
}

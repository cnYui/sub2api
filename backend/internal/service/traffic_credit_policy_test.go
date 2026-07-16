//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrafficCreditPolicyThreshold(t *testing.T) {
	policy := TrafficCreditPolicy{MinimumReserveUSD: 0.01}

	require.False(t, policy.IsDepleted(0.0100000001))
	require.True(t, policy.IsDepleted(0.01))
	require.True(t, policy.IsDepleted(0.0099999999))
	require.InDelta(t, 0.99, policy.AvailableUSD(1.00, 0.01), 1e-10)
	require.Zero(t, policy.AvailableUSD(1.00, 1.00))
	require.Zero(t, policy.AvailableUSD(0.01, 0))
}

func TestPlanTrafficCreditDeductionsSkipsDepletedResidualCards(t *testing.T) {
	now := time.Now()
	policy := TrafficCreditPolicy{MinimumReserveUSD: 0.01}
	deductions, covered := PlanTrafficCreditDeductions([]TrafficCreditBatch{
		{ID: 1, RemainingUSD: 0.01, CreditedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(24 * time.Hour)},
		{ID: 2, RemainingUSD: 0.50, CreditedAt: now.Add(-1 * time.Hour), ExpiresAt: now.Add(48 * time.Hour)},
	}, 0.02, policy)

	require.True(t, covered)
	require.Equal(t, []TrafficCreditDeduction{{CreditID: 2, AmountUSD: 0.02}}, deductions)
}

func TestPlanTrafficCreditReservationsUsesAvailableAfterThreshold(t *testing.T) {
	now := time.Now()
	policy := TrafficCreditPolicy{MinimumReserveUSD: 0.01}
	items, covered := PlanTrafficCreditReservations([]TrafficCreditBatch{
		{ID: 1, RemainingUSD: 0.0100000001, ReservedUSD: 0, CreditedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(24 * time.Hour)},
		{ID: 2, RemainingUSD: 1.00, ReservedUSD: 1.00, CreditedAt: now.Add(-1 * time.Hour), ExpiresAt: now.Add(36 * time.Hour)},
		{ID: 3, RemainingUSD: 0.50, ReservedUSD: 0.10, CreditedAt: now, ExpiresAt: now.Add(48 * time.Hour)},
	}, 0.0300000001, policy)

	require.True(t, covered)
	require.Equal(t, []TrafficCreditReservationItem{{CreditID: 1, ReservedUSD: 0.0100000001}, {CreditID: 3, ReservedUSD: 0.02}}, items)
}

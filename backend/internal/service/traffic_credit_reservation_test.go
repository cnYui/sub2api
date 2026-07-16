//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPlanTrafficCreditReservationsUsesAvailableAmount(t *testing.T) {
	base := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	batches := []TrafficCreditBatch{
		{ID: 1, RemainingUSD: 5, ReservedUSD: 4, ExpiresAt: base.Add(time.Hour)},
		{ID: 2, RemainingUSD: 10, ReservedUSD: 0, ExpiresAt: base.Add(2 * time.Hour)},
	}

	plan, covered := PlanTrafficCreditReservations(batches, 3)

	require.True(t, covered)
	require.Equal(t, []TrafficCreditReservationItem{
		{CreditID: 1, ReservedUSD: 1},
		{CreditID: 2, ReservedUSD: 2},
	}, plan)
}

func TestPlanTrafficCreditReservationsRejectsConcurrentOversell(t *testing.T) {
	plan, covered := PlanTrafficCreditReservations([]TrafficCreditBatch{{
		ID:           1,
		RemainingUSD: 1,
		ReservedUSD:  0.9,
	}}, 0.2)

	require.False(t, covered)
	require.Equal(t, []TrafficCreditReservationItem{{CreditID: 1, ReservedUSD: 0.1}}, plan)
}

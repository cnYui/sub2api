//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrafficCreditReservationMetricsSnapshot(t *testing.T) {
	resetTrafficCreditReservationMetricsForTest()

	recordTrafficCreditPreauthorizationSuccess()
	recordTrafficCreditPreauthorizationRejected(ErrTrafficCreditInsufficient)
	recordTrafficCreditPreauthorizationRejected(ErrTrafficCreditDebtOutstanding)
	recordTrafficCreditReservationUnknown()
	recordTrafficCreditActualExceededReserved()
	recordTrafficCreditStaleReservedReleased(3)

	got := SnapshotTrafficCreditReservationMetrics()

	require.Equal(t, uint64(1), got.PreauthorizationSuccessTotal)
	require.Equal(t, uint64(2), got.PreauthorizationRejectedTotal)
	require.Equal(t, uint64(1), got.DebtBlockedTotal)
	require.Equal(t, uint64(1), got.ReservationUnknownTotal)
	require.Equal(t, uint64(1), got.ActualExceededReservedTotal)
	require.Equal(t, uint64(3), got.StaleReservedReleasedTotal)
}

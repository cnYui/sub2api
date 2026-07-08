//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUsageBillingCommandNormalizeSetsCompletedAtWithoutAffectingFingerprint(t *testing.T) {
	subID := int64(42)
	cmdA := &UsageBillingCommand{
		RequestID:        " req-1 ",
		APIKeyID:         10,
		UserID:           20,
		AccountID:        30,
		SubscriptionID:   &subID,
		SubscriptionCost: 0.75,
		CompletedAt:      time.Date(2026, 7, 8, 0, 1, 0, 0, time.UTC),
	}
	cmdB := &UsageBillingCommand{
		RequestID:        " req-1 ",
		APIKeyID:         10,
		UserID:           20,
		AccountID:        30,
		SubscriptionID:   &subID,
		SubscriptionCost: 0.75,
		CompletedAt:      time.Date(2026, 7, 8, 0, 2, 0, 0, time.UTC),
	}

	cmdA.Normalize()
	cmdB.Normalize()

	require.Equal(t, "req-1", cmdA.RequestID)
	require.False(t, cmdA.CompletedAt.IsZero())
	require.Equal(t, cmdA.RequestFingerprint, cmdB.RequestFingerprint)
}

func TestUsageBillingCommandNormalizeFillsMissingCompletedAt(t *testing.T) {
	cmd := &UsageBillingCommand{RequestID: "req-2"}

	cmd.Normalize()

	require.False(t, cmd.CompletedAt.IsZero())
}

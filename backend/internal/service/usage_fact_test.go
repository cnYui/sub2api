//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageFactPayloadRoundTripPreservesBillingAndLog(t *testing.T) {
	payload := UsageFactPayload{
		BillingCommand: UsageBillingCommand{
			RequestID:       "req-1",
			APIKeyID:        9,
			UserID:          7,
			TrafficPackCost: 0.25,
		},
		UsageLog: UsageLog{
			RequestID:  "req-1",
			APIKeyID:   9,
			UserID:     7,
			ActualCost: 0.25,
		},
	}

	raw, err := EncodeUsageFactPayload(payload)
	require.NoError(t, err)

	got, err := DecodeUsageFactPayload(UsageFactPayloadVersion1, raw)
	require.NoError(t, err)
	require.Equal(t, payload.BillingCommand.RequestID, got.BillingCommand.RequestID)
	require.Equal(t, payload.UsageLog.ActualCost, got.UsageLog.ActualCost)
}

func TestNewUsageFactRejectsMissingRequestID(t *testing.T) {
	_, err := NewUsageFact(UsageFactPayload{})
	require.ErrorIs(t, err, ErrUsageFactRequestIDRequired)
}

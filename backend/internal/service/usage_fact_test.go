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
		OpenAIBilling: &OpenAIUsageBillingSnapshot{
			RateMultiplier:         1.5,
			MissingUsageComponents: []string{"image_input_tokens"},
			BillingIncomplete:      true,
			Components: []OpenAIBillingComponentSnapshot{{
				Component: OpenAIBillingComponent{
					Kind:  "image",
					Model: "gpt-image-2",
				},
			}},
		},
	}

	raw, err := EncodeUsageFactPayload(payload)
	require.NoError(t, err)

	got, err := DecodeUsageFactPayload(UsageFactPayloadVersion1, raw)
	require.NoError(t, err)
	require.Equal(t, payload.BillingCommand.RequestID, got.BillingCommand.RequestID)
	require.Equal(t, payload.UsageLog.ActualCost, got.UsageLog.ActualCost)
	require.NotNil(t, got.OpenAIBilling)
	require.True(t, got.OpenAIBilling.BillingIncomplete)
	require.Equal(t, []string{"image_input_tokens"}, got.OpenAIBilling.MissingUsageComponents)
	require.Equal(t, "gpt-image-2", got.OpenAIBilling.Components[0].Component.Model)
}

func TestNewUsageFactRejectsMissingRequestID(t *testing.T) {
	_, err := NewUsageFact(UsageFactPayload{})
	require.ErrorIs(t, err, ErrUsageFactRequestIDRequired)
}

func TestNewUsageFactUsesBillingAuthorizationID(t *testing.T) {
	authorizationID := int64(88)
	fact, err := NewUsageFact(UsageFactPayload{BillingCommand: UsageBillingCommand{
		RequestID:       "req-authorization",
		APIKeyID:        9,
		UserID:          7,
		AuthorizationID: &authorizationID,
	}})
	require.NoError(t, err)
	require.Equal(t, &authorizationID, fact.AuthorizationID)
}

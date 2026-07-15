package handler

import (
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSanitizePaymentOrderForResponseAddsRefundRetryable(t *testing.T) {
	result := sanitizePaymentOrderForResponse(&dbent.PaymentOrder{
		Status:              service.OrderStatusRefundFailed,
		RefundGatewayStatus: service.RefundGatewayFailed,
	})

	require.NotNil(t, result)
	require.True(t, result.RefundRetryable)
}

func TestSanitizePaymentOrderForResponseAddsHybridFundingFields(t *testing.T) {
	deadline := time.Date(2026, 7, 15, 12, 35, 0, 0, time.UTC)
	compensatedAt := time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC)

	result := sanitizePaymentOrderForResponse(&dbent.PaymentOrder{
		FundingMode:               "mixed",
		BalanceAmount:             6.32,
		GatewayAmount:             73.47,
		PaymentResolutionStatus:   "UNKNOWN",
		PaymentResolutionDeadline: &deadline,
		CompensationAmount:        73.47,
		CompensatedAt:             &compensatedAt,
		RefundBalanceAmount:       5.06,
		RefundGatewayAmount:       58.14,
		RefundBalanceStatus:       service.RefundGatewaySucceeded,
	})

	require.NotNil(t, result)
	require.Equal(t, "mixed", result.FundingMode)
	require.Equal(t, 6.32, result.BalanceAmount)
	require.Equal(t, 73.47, result.GatewayAmount)
	require.Equal(t, "UNKNOWN", result.PaymentResolutionStatus)
	require.Equal(t, &deadline, result.PaymentResolutionDeadline)
	require.Equal(t, 73.47, result.CompensationAmount)
	require.Equal(t, &compensatedAt, result.CompensatedAt)
	require.Equal(t, 5.06, result.RefundBalanceAmount)
	require.Equal(t, 58.14, result.RefundGatewayAmount)
	require.Equal(t, service.RefundGatewaySucceeded, result.RefundBalanceStatus)
}

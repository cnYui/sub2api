package handler

import (
	"testing"

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

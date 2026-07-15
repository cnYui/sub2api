package provider

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestRefundRequestIdentifierUsesPersistedRequestID(t *testing.T) {
	require.Equal(t, "refund-order-7", refundRequestIdentifier(payment.RefundRequest{
		OrderID:   "out-456",
		RequestID: "refund-order-7",
	}))
}

func TestRefundRequestIdentifierHasDeterministicFallback(t *testing.T) {
	require.Equal(t, "refund-out-456", refundRequestIdentifier(payment.RefundRequest{
		OrderID: "out-456",
	}))
}

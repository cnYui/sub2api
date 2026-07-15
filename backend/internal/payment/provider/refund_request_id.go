package provider

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func refundRequestIdentifier(req payment.RefundRequest) string {
	if requestID := strings.TrimSpace(req.RequestID); requestID != "" {
		return requestID
	}
	if orderID := strings.TrimSpace(req.OrderID); orderID != "" {
		return "refund-" + orderID
	}
	if tradeNo := strings.TrimSpace(req.TradeNo); tradeNo != "" {
		return "refund-" + tradeNo
	}
	return ""
}

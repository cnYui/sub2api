package service

import (
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const defaultBalanceRechargeMultiplier = 1.0

var refundBusinessLocation = time.FixedZone("UTC+8", 8*60*60)

func normalizeBalanceRechargeMultiplier(multiplier float64) float64 {
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
		return defaultBalanceRechargeMultiplier
	}
	return multiplier
}

func calculateCreditedBalance(paymentAmount, multiplier float64) float64 {
	return decimal.NewFromFloat(paymentAmount).
		Round(2).
		InexactFloat64()
}

func calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount float64, currency string) float64 {
	if orderAmount <= 0 || payAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	if math.Abs(refundAmount-orderAmount) <= paymentAmountToleranceForCurrency(currency) {
		return decimal.NewFromFloat(payAmount).Round(fractionDigits).InexactFloat64()
	}
	return decimal.NewFromFloat(payAmount).
		Mul(decimal.NewFromFloat(refundAmount)).
		Div(decimal.NewFromFloat(orderAmount)).
		Round(fractionDigits).
		InexactFloat64()
}

func refundCalendarDayIndex(startsAt, now time.Time) int {
	start := startsAt.In(refundBusinessLocation)
	current := now.In(refundBusinessLocation)
	startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	currentDate := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC)
	return int(currentDate.Sub(startDate)/(24*time.Hour)) + 1
}

func calculateSubscriptionRefundAmount(orderAmount float64, subscriptionDays int, startsAt, now time.Time) float64 {
	if orderAmount <= 0 || subscriptionDays <= 0 {
		return 0
	}
	usedDays := refundCalendarDayIndex(startsAt, now)
	if usedDays < 1 {
		usedDays = 1
	}
	if usedDays >= subscriptionDays {
		return 0
	}
	remainingDays := subscriptionDays - usedDays
	return decimal.NewFromFloat(orderAmount).
		Mul(decimal.NewFromInt(int64(remainingDays))).
		Div(decimal.NewFromInt(int64(subscriptionDays))).
		Round(1).
		InexactFloat64()
}

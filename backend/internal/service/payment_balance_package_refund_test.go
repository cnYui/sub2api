package service

import (
	"math"
	"testing"
	"time"
)

func TestRefundTimeRatio(t *testing.T) {
	startsAt := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := startsAt.Add(28 * 24 * time.Hour)
	cases := []struct {
		name string
		now  time.Time
		want float64
	}{
		{name: "before start", now: startsAt.Add(-time.Hour), want: 0},
		{name: "at start", now: startsAt, want: 0},
		{name: "halfway", now: startsAt.Add(14 * 24 * time.Hour), want: 0.5},
		{name: "at expiry", now: expiresAt, want: 1},
		{name: "after expiry", now: expiresAt.Add(time.Hour), want: 1},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := refundTimeRatio(startsAt, expiresAt, tt.now); math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("refundTimeRatio() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClampRefundRatio(t *testing.T) {
	cases := []struct {
		input float64
		want  float64
	}{
		{input: -0.2, want: 0},
		{input: 0.25, want: 0.25},
		{input: 1.5, want: 1},
		{input: math.NaN(), want: 1},
	}
	for _, tt := range cases {
		if got := clampRefundRatio(tt.input); math.Abs(got-tt.want) > 1e-9 {
			t.Fatalf("clampRefundRatio(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestBalancePackageRefundFormula(t *testing.T) {
	cases := []struct {
		name          string
		timeRatio     float64
		usedQuota     float64
		totalQuota    float64
		purchasePrice float64
		wantRatio     float64
		wantRefund    float64
	}{
		{name: "time dominates", timeRatio: 0.6, usedQuota: 20, totalQuota: 100, purchasePrice: 99, wantRatio: 0.6, wantRefund: 39.6},
		{name: "usage dominates", timeRatio: 0.2, usedQuota: 70, totalQuota: 100, purchasePrice: 99, wantRatio: 0.7, wantRefund: 29.7},
		{name: "fully consumed", timeRatio: 0.2, usedQuota: 150, totalQuota: 100, purchasePrice: 99, wantRatio: 1, wantRefund: 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, consumptionRatio, refund := calculateBalancePackageRefundAmounts(tt.purchasePrice, tt.timeRatio, tt.usedQuota, tt.totalQuota)
			if math.Abs(consumptionRatio-tt.wantRatio) > 1e-9 {
				t.Fatalf("consumption ratio = %v, want %v", consumptionRatio, tt.wantRatio)
			}
			if math.Abs(refund-tt.wantRefund) > 1e-9 {
				t.Fatalf("refund = %v, want %v", refund, tt.wantRefund)
			}
		})
	}
}

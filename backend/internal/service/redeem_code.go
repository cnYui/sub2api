package service

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type RedeemCode struct {
	ID        int64
	Code      string
	Type      string
	Value     float64
	Status    string
	UsedBy    *int64
	UsedAt    *time.Time
	Notes     string
	CreatedAt time.Time
	ExpiresAt *time.Time

	GroupID      *int64
	ValidityDays int

	// BalancePackagePlanID 只对 RedeemTypeBalancePackage 有意义。
	BalancePackagePlanID *int64

	User               *User
	Group              *Group
	BalancePackagePlan *RedeemBalancePackagePlan
}

// RedeemBalancePackagePlan 是兑换码绑定的余额套餐档位快照（只读展示用）。
// 字段取自 balance_package_plans 当前值，不是下单时的冻结值。
type RedeemBalancePackagePlan struct {
	ID                  int64
	Code                string
	Name                string
	PriceCNY            float64
	WeeklyCreditUSD     float64
	ValidityDays        int
	RefreshCount        int
	RefreshIntervalDays int
	ForSale             bool
}

func (r *RedeemCode) IsUsed() bool {
	return r.Status == StatusUsed
}

func (r *RedeemCode) IsExpired() bool {
	return r.IsExpiredAt(time.Now())
}

func (r *RedeemCode) IsExpiredAt(now time.Time) bool {
	if r == nil {
		return false
	}
	if r.Status == StatusExpired {
		return true
	}
	return r.Status == StatusUnused && r.ExpiresAt != nil && !r.ExpiresAt.After(now)
}

func (r *RedeemCode) CanUse() bool {
	return r.Status == StatusUnused && !r.IsExpired()
}

func GenerateRedeemCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

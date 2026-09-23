package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// AdminRedeemCard 是后台「兑换卡」列表里的一行。
type AdminRedeemCard struct {
	ID           int64                     `json:"id"`
	Token        string                    `json:"token"`
	RedeemCodeID int64                     `json:"redeem_code_id"`
	Theme        string                    `json:"theme"`
	Content      service.RedeemCardContent `json:"content"`
	ViewCount    int                       `json:"view_count"`
	LastViewedAt *time.Time                `json:"last_viewed_at,omitempty"`
	CreatedAt    time.Time                 `json:"created_at"`
	UpdatedAt    time.Time                 `json:"updated_at"`
	Code         string                    `json:"code"`
	CodeType     string                    `json:"code_type"`
	CodeValue    float64                   `json:"code_value"`
	CodeStatus   string                    `json:"code_status"`
}

func AdminRedeemCardFromService(card *service.RedeemCard) *AdminRedeemCard {
	if card == nil {
		return nil
	}
	return &AdminRedeemCard{
		ID:           card.ID,
		Token:        card.Token,
		RedeemCodeID: card.RedeemCodeID,
		Theme:        card.Theme,
		Content:      card.Content,
		ViewCount:    card.ViewCount,
		LastViewedAt: card.LastViewedAt,
		CreatedAt:    card.CreatedAt,
		UpdatedAt:    card.UpdatedAt,
		Code:         card.Code,
		CodeType:     card.CodeType,
		CodeValue:    card.CodeValue,
		CodeStatus:   card.CodeStatus,
	}
}

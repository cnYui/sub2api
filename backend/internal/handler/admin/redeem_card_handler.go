package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RedeemCardHandler 管理兑换卡：为兑换码生成 3D 卡片分享链接，以及所有卡片共用的卡面信息。
type RedeemCardHandler struct {
	redeemCardService *service.RedeemCardService
}

func NewRedeemCardHandler(redeemCardService *service.RedeemCardService) *RedeemCardHandler {
	return &RedeemCardHandler{redeemCardService: redeemCardService}
}

type saveRedeemCardRequest struct {
	RedeemCodeID int64                     `json:"redeem_code_id" binding:"required,gt=0"`
	Theme        string                    `json:"theme"`
	Content      service.RedeemCardContent `json:"content"`
}

// List GET /api/v1/admin/redeem-cards
func (h *RedeemCardHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	cards, total, err := h.redeemCardService.List(c.Request.Context(), page, pageSize, c.Query("search"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.AdminRedeemCard, 0, len(cards))
	for i := range cards {
		out = append(out, *dto.AdminRedeemCardFromService(&cards[i]))
	}
	response.Paginated(c, out, total, page, pageSize)
}

// GetByCode GET /api/v1/admin/redeem-cards/by-code/:code_id，没有卡片时 data 为 null。
func (h *RedeemCardHandler) GetByCode(c *gin.Context) {
	codeID, err := strconv.ParseInt(c.Param("code_id"), 10, 64)
	if err != nil || codeID <= 0 {
		response.BadRequest(c, "Invalid redeem code ID")
		return
	}
	card, err := h.redeemCardService.GetByCodeID(c.Request.Context(), codeID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminRedeemCardFromService(card))
}

// Save POST /api/v1/admin/redeem-cards：兑换码没有卡片就新建，有就更新卡面（链接不变）。
func (h *RedeemCardHandler) Save(c *gin.Context) {
	var req saveRedeemCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input := service.RedeemCardSaveInput{
		RedeemCodeID: req.RedeemCodeID,
		Theme:        req.Theme,
		Content:      req.Content,
	}
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		input.AdminID = subject.UserID
	}
	card, err := h.redeemCardService.Save(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminRedeemCardFromService(card))
}

// Delete DELETE /api/v1/admin/redeem-cards/:id：撤销分享链接，兑换码不受影响。
func (h *RedeemCardHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid redeem card ID")
		return
	}
	if err := h.redeemCardService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "redeem card deleted"})
}

// GetProfile GET /api/v1/admin/redeem-card-profile
func (h *RedeemCardHandler) GetProfile(c *gin.Context) {
	profile, err := h.redeemCardService.GetProfile(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}

// UpdateProfile PUT /api/v1/admin/redeem-card-profile
func (h *RedeemCardHandler) UpdateProfile(c *gin.Context) {
	var req service.RedeemCardProfile
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	profile, err := h.redeemCardService.UpdateProfile(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profile)
}

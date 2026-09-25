package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RedeemCardHandler 为 /card/<token> 页面提供兑换卡数据，匿名可访问。
type RedeemCardHandler struct {
	redeemCardService *service.RedeemCardService
}

func NewRedeemCardHandler(redeemCardService *service.RedeemCardService) *RedeemCardHandler {
	return &RedeemCardHandler{redeemCardService: redeemCardService}
}

// GetByToken GET /api/v1/redeem-cards/:token
func (h *RedeemCardHandler) GetByToken(c *gin.Context) {
	view, err := h.redeemCardService.GetPublic(c.Request.Context(), c.Param("token"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// 响应里有兑换码明文，不能被任何中间层缓存，也不该被搜索引擎收录。
	c.Header("Cache-Control", "no-store")
	c.Header("X-Robots-Tag", "noindex, nofollow")
	response.Success(c, view)
}

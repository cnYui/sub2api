package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterRedeemCardRoutes 注册兑换卡公开路由：用户打开 /card/<token> 时前端调它取卡面数据。
//
// 匿名可访问，按 IP 限流；token 是 128 位随机数，枚举不可行。
// 与模型广场一样挂 OptionalJWT + BackendModeUserGuard：backend 模式下只有管理员能看。
func RegisterRedeemCardRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	optionalJWT middleware.OptionalJWTAuthMiddleware,
	settingService *service.SettingService,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	cards := v1.Group("/redeem-cards")
	cards.Use(panelRateLimiter.PublicIP())
	cards.Use(gin.HandlerFunc(optionalJWT))
	cards.Use(middleware.BackendModeUserGuard(settingService))
	{
		cards.GET("/:token", h.RedeemCard.GetByToken)
	}
}

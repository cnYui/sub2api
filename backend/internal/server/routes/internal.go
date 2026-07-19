package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterInternalRoutes(r *gin.Engine, h *handler.Handlers) {
	if h == nil || h.InternalUsageEvent == nil {
		return
	}
	internal := r.Group("/api/internal")
	{
		internal.POST("/usage-events", h.InternalUsageEvent.ReceiveUsageEvent)
	}
}

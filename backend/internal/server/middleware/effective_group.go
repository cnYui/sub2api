package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type EffectiveGroupResolver interface {
	ResolveEffectiveGroupForRequest(ctx context.Context, userID int64, path string, forcePlatform string) (*service.EffectiveGroupResult, error)
}

func ResolveEffectiveGroup(resolver EffectiveGroupResolver, writeError GatewayErrorWriter) gin.HandlerFunc {
	if writeError == nil {
		writeError = AnthropicErrorWriter
	}
	return func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.GroupID != nil {
			c.Next()
			return
		}
		if resolver == nil {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnassigned)
			writeError(c, http.StatusForbidden, "API Key is not assigned to any group and cannot be resolved automatically.")
			c.Abort()
			return
		}

		forcePlatform, _ := c.Request.Context().Value(ctxkey.ForcePlatform).(string)
		result, err := resolver.ResolveEffectiveGroupForRequest(c.Request.Context(), apiKey.UserID, c.Request.URL.Path, forcePlatform)
		if err != nil {
			status, message := effectiveGroupErrorResponse(err)
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable)
			writeError(c, status, message)
			c.Abort()
			return
		}
		if result == nil || result.Group == nil {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable)
			writeError(c, http.StatusForbidden, "No available group for this API Key.")
			c.Abort()
			return
		}

		requestKey := *apiKey
		groupID := result.Group.ID
		requestKey.GroupID = &groupID
		requestKey.Group = result.Group

		c.Set(string(ContextKeyAPIKey), &requestKey)
		if result.Subscription != nil {
			c.Set(string(ContextKeySubscription), result.Subscription)
		}
		setGroupContext(c, result.Group)
		c.Next()
	}
}

func effectiveGroupErrorResponse(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrNoOpenAIEntitlement):
		return http.StatusForbidden, "请先购买套餐或 GPT 流量包"
	case errors.Is(err, service.ErrOpenAITrafficGroupUnavailable):
		return http.StatusServiceUnavailable, "OpenAI 流量包入口分组不可用"
	default:
		return http.StatusInternalServerError, "Failed to resolve API Key group"
	}
}

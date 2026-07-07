package handler

import (
	"context"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type gatewayBillingEligibilityChecker interface {
	CheckBillingEligibility(ctx context.Context, user *service.User, apiKey *service.APIKey, group *service.Group, subscription *service.UserSubscription, platform string) error
}

type gatewayBillingFailure struct {
	Status     int
	Code       string
	Message    string
	RetryAfter int
	Err        error
}

func (f gatewayBillingFailure) RetryAfterString() string {
	if f.RetryAfter <= 0 {
		return ""
	}
	return strconv.Itoa(f.RetryAfter)
}

func checkGatewayBillingEligibility(ctx context.Context, c *gin.Context, checker gatewayBillingEligibilityChecker, apiKey *service.APIKey, subscription *service.UserSubscription) *gatewayBillingFailure {
	return checkGatewayBillingEligibilityForPlatform(ctx, c, checker, apiKey, subscription, service.QuotaPlatform(ctx, apiKey))
}

func checkGatewayBillingEligibilityForPlatform(ctx context.Context, c *gin.Context, checker gatewayBillingEligibilityChecker, apiKey *service.APIKey, subscription *service.UserSubscription, platform string) *gatewayBillingFailure {
	if checker == nil {
		return &gatewayBillingFailure{
			Status:  503,
			Code:    "billing_service_error",
			Message: "Billing service temporarily unavailable. Please retry later.",
		}
	}
	err := checker.CheckBillingEligibility(ctx, apiKey.User, apiKey, apiKey.Group, subscription, platform)
	if err == nil {
		return nil
	}

	status, code, message, retryAfter := billingErrorDetails(err)
	failure := &gatewayBillingFailure{
		Status:     status,
		Code:       code,
		Message:    message,
		RetryAfter: retryAfter,
		Err:        err,
	}
	if retryAfter > 0 {
		c.Header("Retry-After", failure.RetryAfterString())
	}
	return failure
}

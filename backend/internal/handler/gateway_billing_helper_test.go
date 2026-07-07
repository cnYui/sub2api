//go:build unit

package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayBillingCheckerStub struct {
	called bool
	err    error
}

func (s *gatewayBillingCheckerStub) CheckBillingEligibility(ctx context.Context, user *service.User, apiKey *service.APIKey, group *service.Group, subscription *service.UserSubscription, platform string) error {
	s.called = true
	return s.err
}

func TestCheckGatewayBillingEligibilitySetsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resetAt := time.Now().Add(90 * time.Second).UTC().Format(time.RFC3339)
	checker := &gatewayBillingCheckerStub{
		err: service.ErrUserPlatformDailyQuotaExhausted.WithMetadata(map[string]string{
			"window_resets_at": resetAt,
		}),
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	apiKey := &service.APIKey{
		User: &service.User{ID: 7},
		Group: &service.Group{
			ID:       9,
			Platform: service.PlatformOpenAI,
		},
	}

	failure := checkGatewayBillingEligibility(c.Request.Context(), c, checker, apiKey, nil)

	require.True(t, checker.called)
	require.NotNil(t, failure)
	require.Equal(t, 429, failure.Status)
	require.Equal(t, "rate_limit_exceeded", failure.Code)
	require.NotEmpty(t, failure.Message)
	require.Greater(t, failure.RetryAfter, 0)
	require.Equal(t, failure.RetryAfterString(), w.Header().Get("Retry-After"))
}

func TestCheckGatewayBillingEligibilityReturnsNilWhenEligible(t *testing.T) {
	gin.SetMode(gin.TestMode)

	checker := &gatewayBillingCheckerStub{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	apiKey := &service.APIKey{
		User:  &service.User{ID: 7},
		Group: &service.Group{Platform: service.PlatformOpenAI},
	}

	failure := checkGatewayBillingEligibility(c.Request.Context(), c, checker, apiKey, nil)

	require.True(t, checker.called)
	require.Nil(t, failure)
	require.Empty(t, w.Header().Get("Retry-After"))
}

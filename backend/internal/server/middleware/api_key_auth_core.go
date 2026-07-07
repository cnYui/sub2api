package middleware

import (
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type apiKeyAuthFailure struct {
	status  int
	code    string
	message string
}

type apiKeyAuthOptions struct {
	extractAPIKey func(c *gin.Context) (string, *apiKeyAuthFailure)
	writeError    func(c *gin.Context, failure apiKeyAuthFailure)
	skipBilling   func(path string) bool
}

func authenticateAPIKeyCore(c *gin.Context, apiKeyService *service.APIKeyService, subscriptionService *service.SubscriptionService, cfg *config.Config, opts apiKeyAuthOptions) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	if opts.skipBilling == nil {
		opts.skipBilling = func(string) bool { return false }
	}

	apiKeyString, failure := opts.extractAPIKey(c)
	if failure != nil {
		opts.writeError(c, *failure)
		return
	}
	if apiKeyString == "" {
		opts.writeError(c, apiKeyAuthFailure{status: 401, code: "API_KEY_REQUIRED", message: "API key is required"})
		return
	}

	apiKey, err := apiKeyService.GetByKey(c.Request.Context(), apiKeyString)
	if err != nil {
		if errors.Is(err, service.ErrAPIKeyNotFound) {
			opts.writeError(c, apiKeyAuthFailure{status: 401, code: "INVALID_API_KEY", message: "Invalid API key"})
			return
		}
		opts.writeError(c, apiKeyAuthFailure{status: 500, code: "INTERNAL_ERROR", message: "Failed to validate API key"})
		return
	}

	SetOpsFallbackAPIKey(c, apiKey)

	if !apiKey.IsActive() &&
		apiKey.Status != service.StatusAPIKeyExpired &&
		apiKey.Status != service.StatusAPIKeyQuotaExhausted {
		opts.writeError(c, apiKeyAuthFailure{status: 401, code: "API_KEY_DISABLED", message: "API key is disabled"})
		return
	}
	if failure := validateAPIKeyIPAccess(c, apiKey, cfg); failure != nil {
		opts.writeError(c, *failure)
		return
	}
	if apiKey.User == nil {
		opts.writeError(c, apiKeyAuthFailure{status: 401, code: "USER_NOT_FOUND", message: "User associated with API key not found"})
		return
	}
	if !apiKey.User.IsActive() {
		opts.writeError(c, apiKeyAuthFailure{status: 401, code: "USER_INACTIVE", message: "User account is not active"})
		return
	}
	if failure := validateAPIKeyGroupAccess(c, apiKey); failure != nil {
		opts.writeError(c, *failure)
		return
	}

	if cfg.RunMode == config.RunModeSimple {
		setAuthenticatedAPIKeyContext(c, apiKey, nil)
		_ = apiKeyService.TouchLastUsed(c.Request.Context(), apiKey.ID)
		c.Next()
		return
	}

	var subscription *service.UserSubscription
	isSubscriptionType := apiKey.Group != nil && apiKey.Group.IsSubscriptionType()
	if isSubscriptionType && subscriptionService != nil {
		sub, subErr := subscriptionService.GetActiveSubscription(
			c.Request.Context(),
			apiKey.User.ID,
			apiKey.Group.ID,
		)
		if subErr != nil {
			if !errors.Is(subErr, service.ErrSubscriptionNotFound) {
				opts.writeError(c, apiKeyAuthFailure{status: 500, code: "SUBSCRIPTION_LOAD_FAILED", message: "Failed to load subscription"})
				return
			}
		} else {
			subscription = sub
		}
	}

	if !opts.skipBilling(c.Request.URL.Path) {
		if failure := validateAPIKeyBillingState(apiKey); failure != nil {
			opts.writeError(c, *failure)
			return
		}
		if subscription != nil {
			if failure := validateSubscriptionForAuth(subscriptionService, subscription, apiKey.Group); failure != nil {
				opts.writeError(c, *failure)
				return
			}
		}
	}

	setAuthenticatedAPIKeyContext(c, apiKey, subscription)
	_ = apiKeyService.TouchLastUsed(c.Request.Context(), apiKey.ID)
	c.Next()
}

func validateAPIKeyIPAccess(c *gin.Context, apiKey *service.APIKey, cfg *config.Config) *apiKeyAuthFailure {
	if len(apiKey.IPWhitelist) == 0 && len(apiKey.IPBlacklist) == 0 {
		return nil
	}

	clientIP := ip.GetTrustedClientIP(c)
	if cfg.TrustForwardedIPForAPIKeyACL() {
		clientIP = ip.GetClientIP(c)
	}
	allowed, _ := ip.CheckIPRestrictionWithCompiledRules(clientIP, apiKey.CompiledIPWhitelist, apiKey.CompiledIPBlacklist)
	if allowed {
		return nil
	}

	if clientIP == "" {
		clientIP = "unknown"
	}
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonIPRestriction)
	return &apiKeyAuthFailure{
		status:  403,
		code:    "ACCESS_DENIED",
		message: fmt.Sprintf("Access denied. Your IP is %s", clientIP),
	}
}

func validateAPIKeyGroupAccess(c *gin.Context, apiKey *service.APIKey) *apiKeyAuthFailure {
	code, message, ok := validateAPIKeyGroupAvailable(apiKey)
	if !ok {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable)
		return &apiKeyAuthFailure{status: 403, code: code, message: message}
	}
	if !validateAPIKeyGroupAllowed(apiKey) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable)
		return &apiKeyAuthFailure{
			status:  403,
			code:    "GROUP_NOT_ALLOWED",
			message: "API Key 所属专属分组不再允许当前用户使用",
		}
	}
	return nil
}

func validateAPIKeyBillingState(apiKey *service.APIKey) *apiKeyAuthFailure {
	switch apiKey.Status {
	case service.StatusAPIKeyQuotaExhausted:
		return &apiKeyAuthFailure{status: 429, code: "API_KEY_QUOTA_EXHAUSTED", message: "API key 额度已用完"}
	case service.StatusAPIKeyExpired:
		return &apiKeyAuthFailure{status: 403, code: "API_KEY_EXPIRED", message: "API key 已过期"}
	}
	if apiKey.IsExpired() {
		return &apiKeyAuthFailure{status: 403, code: "API_KEY_EXPIRED", message: "API key 已过期"}
	}
	if apiKey.IsQuotaExhausted() {
		return &apiKeyAuthFailure{status: 429, code: "API_KEY_QUOTA_EXHAUSTED", message: "API key 额度已用完"}
	}
	return nil
}

func validateSubscriptionForAuth(subscriptionService *service.SubscriptionService, subscription *service.UserSubscription, group *service.Group) *apiKeyAuthFailure {
	needsMaintenance, err := subscriptionService.ValidateAndCheckLimits(subscription, group)
	if err != nil {
		if isSubscriptionUsageLimitError(err) {
			err = nil
		}
	}
	if err != nil {
		return &apiKeyAuthFailure{status: 403, code: "SUBSCRIPTION_INVALID", message: err.Error()}
	}
	if needsMaintenance {
		maintenanceCopy := *subscription
		subscriptionService.DoWindowMaintenance(&maintenanceCopy)
	}
	return nil
}

func setAuthenticatedAPIKeyContext(c *gin.Context, apiKey *service.APIKey, subscription *service.UserSubscription) {
	if subscription != nil {
		c.Set(string(ContextKeySubscription), subscription)
	}
	c.Set(string(ContextKeyAPIKey), apiKey)
	c.Set(string(ContextKeyUser), AuthSubject{
		UserID:      apiKey.User.ID,
		Concurrency: apiKey.User.Concurrency,
	})
	c.Set(string(ContextKeyUserRole), apiKey.User.Role)
	setGroupContext(c, apiKey.Group)
}

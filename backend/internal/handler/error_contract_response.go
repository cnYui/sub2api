package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain/errorcontract"
	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
	"github.com/gin-gonic/gin"
)

func writeOpenAIContractError(c *gin.Context, fact errorcontract.Fact) {
	setErrorContractHeaders(c, fact)

	c.JSON(fact.HTTPStatus, gin.H{"error": contractErrorBody(c, fact)})
}

func writeAnthropicContractError(c *gin.Context, fact errorcontract.Fact) {
	setErrorContractHeaders(c, fact)
	c.JSON(fact.HTTPStatus, gin.H{
		"type":  "error",
		"error": contractErrorBody(c, fact),
	})
}

func writeGoogleContractError(c *gin.Context, fact errorcontract.Fact) {
	setErrorContractHeaders(c, fact)
	metadata := map[string]string{
		"error_id":  string(fact.ID),
		"retryable": strconv.FormatBool(fact.Retryable),
	}
	if fact.RetryAfterSeconds != nil {
		metadata["retry_after"] = strconv.Itoa(*fact.RetryAfterSeconds)
	}
	if requestID := strings.TrimSpace(c.Writer.Header().Get("X-Request-ID")); requestID != "" {
		metadata["request_id"] = requestID
	}
	c.JSON(fact.HTTPStatus, gin.H{
		"error": gin.H{
			"code":    fact.HTTPStatus,
			"message": fact.Message,
			"status":  googleapi.HTTPStatusToGoogleStatus(fact.HTTPStatus),
			"details": []gin.H{{
				"@type":    "type.googleapis.com/google.rpc.ErrorInfo",
				"reason":   fact.Code,
				"domain":   "sub2api",
				"metadata": metadata,
			}},
		},
	})
}

func contractErrorBody(c *gin.Context, fact errorcontract.Fact) gin.H {
	errorBody := gin.H{
		"type":         fact.Type,
		"message":      fact.Message,
		"error_id":     fact.ID,
		"sub2api_code": fact.Code,
		"retryable":    fact.Retryable,
	}
	if fact.RetryAfterSeconds != nil {
		errorBody["retry_after"] = *fact.RetryAfterSeconds
	}
	if requestID := strings.TrimSpace(c.Writer.Header().Get("X-Request-ID")); requestID != "" {
		errorBody["request_id"] = requestID
	}
	return errorBody
}

func setErrorContractHeaders(c *gin.Context, fact errorcontract.Fact) {
	if c == nil {
		return
	}
	c.Header("X-Sub2API-Error-ID", string(fact.ID))
	c.Header("X-Sub2API-Error-Code", string(fact.Code))
	c.Header("X-Sub2API-Retryable", strconv.FormatBool(fact.Retryable))
	if fact.RetryAfterSeconds != nil {
		c.Header("Retry-After", strconv.Itoa(*fact.RetryAfterSeconds))
	}
}

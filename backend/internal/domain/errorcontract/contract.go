// Package errorcontract defines stable public error semantics.
package errorcontract

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type ID string

const (
	IDNoAvailableUpstream            ID = "S2A-5001"
	IDUpstreamCredentialsUnavailable ID = "S2A-5002"
	IDUpstreamAuthenticationFailed   ID = "S2A-5003"
	IDUpstreamRateLimited            ID = "S2A-5004"
	IDUpstreamModelUnavailable       ID = "S2A-5005"
	IDUpstreamOverloaded             ID = "S2A-5006"
	IDUpstreamTimeout                ID = "S2A-5007"
	IDUpstreamConnectionFailed       ID = "S2A-5008"
	IDUpstreamInvalidResponse        ID = "S2A-5009"
	IDUpstreamAccessDenied           ID = "S2A-5010"
)

type Code string

const (
	CodeNoAvailableUpstream            Code = "NO_AVAILABLE_UPSTREAM"
	CodeUpstreamCredentialsUnavailable Code = "UPSTREAM_CREDENTIALS_UNAVAILABLE"
	CodeUpstreamAuthenticationFailed   Code = "UPSTREAM_AUTHENTICATION_FAILED"
	CodeUpstreamRateLimited            Code = "UPSTREAM_RATE_LIMITED"
	CodeUpstreamModelUnavailable       Code = "UPSTREAM_MODEL_UNAVAILABLE"
	CodeUpstreamOverloaded             Code = "UPSTREAM_OVERLOADED"
	CodeUpstreamTimeout                Code = "UPSTREAM_TIMEOUT"
	CodeUpstreamConnectionFailed       Code = "UPSTREAM_CONNECTION_FAILED"
	CodeUpstreamInvalidResponse        Code = "UPSTREAM_INVALID_RESPONSE"
	CodeUpstreamAccessDenied           Code = "UPSTREAM_ACCESS_DENIED"
)

type FailureClass string

const (
	FailureClassAllAccountsRateLimited   FailureClass = "all_accounts_rate_limited"
	FailureClassCredentialsUnavailable   FailureClass = "credentials_unavailable"
	FailureClassModelUnavailable         FailureClass = "model_unavailable"
	FailureClassUpstreamOverloaded       FailureClass = "upstream_overloaded"
	FailureClassUpstreamConnectionFailed FailureClass = "upstream_connection_failed"
)

type UpstreamInput struct {
	StatusCode        int
	FailureClass      FailureClass
	RetryAfterSeconds *int
}

type Fact struct {
	ID                ID
	Code              Code
	HTTPStatus        int
	Type              string
	Message           string
	Retryable         bool
	RetryAfterSeconds *int
}

// UpstreamInputFromResponse extracts only stable, allowlisted proxy signals.
// Upstream messages remain diagnostics and must not alter public error semantics.
func UpstreamInputFromResponse(statusCode int, headers http.Header, body []byte) UpstreamInput {
	input := UpstreamInput{StatusCode: statusCode}
	if headers != nil {
		input.FailureClass = knownFailureClass(headers.Get("X-CLIProxy-Error-Class"))
		input.RetryAfterSeconds = retryAfterFromHeader(headers)
	}

	var payload struct {
		RetryAfter *int `json:"retry_after"`
		Error      struct {
			Code       string `json:"code"`
			Type       string `json:"type"`
			RetryAfter *int   `json:"retry_after"`
		} `json:"error"`
	}
	if len(body) == 0 || json.Unmarshal(body, &payload) != nil {
		return input
	}
	if input.FailureClass == "" {
		input.FailureClass = knownFailureClass(payload.Error.Code)
	}
	if input.FailureClass == "" {
		input.FailureClass = knownFailureClass(payload.Error.Type)
	}
	if input.RetryAfterSeconds == nil {
		input.RetryAfterSeconds = positiveSeconds(payload.Error.RetryAfter)
	}
	if input.RetryAfterSeconds == nil {
		input.RetryAfterSeconds = positiveSeconds(payload.RetryAfter)
	}
	return input
}

func knownFailureClass(raw string) FailureClass {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "all_accounts_rate_limited", "rate_limit_exceeded", "rate_limit_error":
		return FailureClassAllAccountsRateLimited
	case "credentials_unavailable", "auth_unavailable":
		return FailureClassCredentialsUnavailable
	case "model_unavailable", "model_not_found", "model_not_supported":
		return FailureClassModelUnavailable
	case "upstream_overloaded", "server_error", "internal_server_error":
		return FailureClassUpstreamOverloaded
	case "upstream_connection_failed":
		return FailureClassUpstreamConnectionFailed
	default:
		return ""
	}
}

func retryAfterFromHeader(headers http.Header) *int {
	seconds, err := strconv.Atoi(strings.TrimSpace(headers.Get("Retry-After")))
	if err != nil {
		return nil
	}
	return positiveSeconds(&seconds)
}

func positiveSeconds(value *int) *int {
	if value == nil || *value <= 0 {
		return nil
	}
	seconds := *value
	return &seconds
}

func ClassifyUpstream(input UpstreamInput) Fact {
	switch {
	case input.FailureClass == FailureClassAllAccountsRateLimited || input.StatusCode == http.StatusTooManyRequests:
		return newUpstreamFact(
			IDUpstreamRateLimited,
			CodeUpstreamRateLimited,
			http.StatusTooManyRequests,
			"The upstream service is rate limited. Please retry later.",
			true,
			input.RetryAfterSeconds,
		)
	case input.FailureClass == FailureClassCredentialsUnavailable:
		return newUpstreamFact(
			IDUpstreamCredentialsUnavailable,
			CodeUpstreamCredentialsUnavailable,
			http.StatusServiceUnavailable,
			"No upstream credentials are currently available. Please retry later.",
			true,
			nil,
		)
	case input.StatusCode == http.StatusUnauthorized:
		return newUpstreamFact(
			IDUpstreamAuthenticationFailed,
			CodeUpstreamAuthenticationFailed,
			http.StatusBadGateway,
			"The upstream service rejected its credentials.",
			false,
			nil,
		)
	case input.StatusCode == http.StatusForbidden:
		return newUpstreamFact(
			IDUpstreamAccessDenied,
			CodeUpstreamAccessDenied,
			http.StatusBadGateway,
			"The upstream service denied the request.",
			false,
			nil,
		)
	case input.FailureClass == FailureClassModelUnavailable:
		return newUpstreamFact(
			IDUpstreamModelUnavailable,
			CodeUpstreamModelUnavailable,
			http.StatusServiceUnavailable,
			"The requested model is not currently available from the upstream service.",
			true,
			nil,
		)
	case input.FailureClass == FailureClassUpstreamOverloaded || input.StatusCode == http.StatusInternalServerError || input.StatusCode == 529:
		return newUpstreamFact(
			IDUpstreamOverloaded,
			CodeUpstreamOverloaded,
			http.StatusServiceUnavailable,
			"The upstream service is temporarily overloaded. Please retry later.",
			true,
			nil,
		)
	case input.StatusCode == http.StatusServiceUnavailable:
		return newUpstreamFact(
			IDNoAvailableUpstream,
			CodeNoAvailableUpstream,
			http.StatusServiceUnavailable,
			"No upstream service is currently available. Please retry later.",
			true,
			nil,
		)
	case input.StatusCode == http.StatusGatewayTimeout:
		return newUpstreamFact(
			IDUpstreamTimeout,
			CodeUpstreamTimeout,
			http.StatusGatewayTimeout,
			"The upstream service did not respond in time. Please retry later.",
			true,
			nil,
		)
	case input.FailureClass == FailureClassUpstreamConnectionFailed || input.StatusCode == http.StatusBadGateway:
		return newUpstreamFact(
			IDUpstreamConnectionFailed,
			CodeUpstreamConnectionFailed,
			http.StatusBadGateway,
			"The gateway could not connect to the upstream service.",
			true,
			nil,
		)
	default:
		return newUpstreamFact(
			IDUpstreamInvalidResponse,
			CodeUpstreamInvalidResponse,
			http.StatusBadGateway,
			"The upstream service returned an invalid response.",
			false,
			nil,
		)
	}
}

func newUpstreamFact(id ID, code Code, status int, message string, retryable bool, retryAfter *int) Fact {
	if retryAfter != nil && *retryAfter <= 0 {
		retryAfter = nil
	}
	errType := "upstream_error"
	if code == CodeUpstreamRateLimited {
		errType = "rate_limit_error"
	}
	return Fact{
		ID:                id,
		Code:              code,
		HTTPStatus:        status,
		Type:              errType,
		Message:           message,
		Retryable:         retryable,
		RetryAfterSeconds: retryAfter,
	}
}

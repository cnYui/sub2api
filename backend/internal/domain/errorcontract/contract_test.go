package errorcontract

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyUpstreamPreservesFailureSemantics(t *testing.T) {
	retryAfter := 30
	tests := []struct {
		name           string
		input          UpstreamInput
		wantID         ID
		wantCode       Code
		wantHTTP       int
		wantMessage    string
		wantRetryable  bool
		wantRetryAfter *int
	}{
		{
			name:           "all accounts rate limited",
			input:          UpstreamInput{FailureClass: FailureClassAllAccountsRateLimited, RetryAfterSeconds: &retryAfter},
			wantID:         IDUpstreamRateLimited,
			wantCode:       CodeUpstreamRateLimited,
			wantHTTP:       http.StatusTooManyRequests,
			wantMessage:    "The upstream service is rate limited. Please retry later.",
			wantRetryable:  true,
			wantRetryAfter: &retryAfter,
		},
		{
			name:          "credentials unavailable",
			input:         UpstreamInput{FailureClass: FailureClassCredentialsUnavailable},
			wantID:        IDUpstreamCredentialsUnavailable,
			wantCode:      CodeUpstreamCredentialsUnavailable,
			wantHTTP:      http.StatusServiceUnavailable,
			wantMessage:   "No upstream credentials are currently available. Please retry later.",
			wantRetryable: true,
		},
		{
			name:          "upstream authentication failed",
			input:         UpstreamInput{StatusCode: http.StatusUnauthorized},
			wantID:        IDUpstreamAuthenticationFailed,
			wantCode:      CodeUpstreamAuthenticationFailed,
			wantHTTP:      http.StatusBadGateway,
			wantMessage:   "The upstream service rejected its credentials.",
			wantRetryable: false,
		},
		{
			name:          "upstream access denied",
			input:         UpstreamInput{StatusCode: http.StatusForbidden},
			wantID:        IDUpstreamAccessDenied,
			wantCode:      CodeUpstreamAccessDenied,
			wantHTTP:      http.StatusBadGateway,
			wantMessage:   "The upstream service denied the request.",
			wantRetryable: false,
		},
		{
			name:          "upstream model unavailable",
			input:         UpstreamInput{FailureClass: FailureClassModelUnavailable},
			wantID:        IDUpstreamModelUnavailable,
			wantCode:      CodeUpstreamModelUnavailable,
			wantHTTP:      http.StatusServiceUnavailable,
			wantMessage:   "The requested model is not currently available from the upstream service.",
			wantRetryable: true,
		},
		{
			name:          "upstream timeout",
			input:         UpstreamInput{StatusCode: http.StatusGatewayTimeout},
			wantID:        IDUpstreamTimeout,
			wantCode:      CodeUpstreamTimeout,
			wantHTTP:      http.StatusGatewayTimeout,
			wantMessage:   "The upstream service did not respond in time. Please retry later.",
			wantRetryable: true,
		},
		{
			name:          "upstream server error",
			input:         UpstreamInput{StatusCode: http.StatusInternalServerError},
			wantID:        IDUpstreamOverloaded,
			wantCode:      CodeUpstreamOverloaded,
			wantHTTP:      http.StatusServiceUnavailable,
			wantMessage:   "The upstream service is temporarily overloaded. Please retry later.",
			wantRetryable: true,
		},
		{
			name:          "upstream unavailable",
			input:         UpstreamInput{StatusCode: http.StatusServiceUnavailable},
			wantID:        IDNoAvailableUpstream,
			wantCode:      CodeNoAvailableUpstream,
			wantHTTP:      http.StatusServiceUnavailable,
			wantMessage:   "No upstream service is currently available. Please retry later.",
			wantRetryable: true,
		},
		{
			name:          "upstream connection failure",
			input:         UpstreamInput{FailureClass: FailureClassUpstreamConnectionFailed},
			wantID:        IDUpstreamConnectionFailed,
			wantCode:      CodeUpstreamConnectionFailed,
			wantHTTP:      http.StatusBadGateway,
			wantMessage:   "The gateway could not connect to the upstream service.",
			wantRetryable: true,
		},
		{
			name:          "upstream invalid response",
			input:         UpstreamInput{StatusCode: http.StatusTeapot},
			wantID:        IDUpstreamInvalidResponse,
			wantCode:      CodeUpstreamInvalidResponse,
			wantHTTP:      http.StatusBadGateway,
			wantMessage:   "The upstream service returned an invalid response.",
			wantRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyUpstream(tt.input)

			require.Equal(t, tt.wantID, got.ID)
			require.Equal(t, tt.wantCode, got.Code)
			require.Equal(t, tt.wantHTTP, got.HTTPStatus)
			require.Equal(t, tt.wantMessage, got.Message)
			require.Equal(t, tt.wantRetryable, got.Retryable)
			require.Equal(t, tt.wantRetryAfter, got.RetryAfterSeconds)
		})
	}
}

func TestUpstreamInputFromResponseRecognizesStableFailureClasses(t *testing.T) {
	t.Run("CLIProxy all accounts rate limited", func(t *testing.T) {
		headers := make(http.Header)
		headers.Set("X-CLIProxy-Error-Class", "all_accounts_rate_limited")
		headers.Set("Retry-After", "17")

		input := UpstreamInputFromResponse(http.StatusServiceUnavailable, headers, nil)

		require.Equal(t, FailureClassAllAccountsRateLimited, input.FailureClass)
		require.NotNil(t, input.RetryAfterSeconds)
		require.Equal(t, 17, *input.RetryAfterSeconds)
	})

	t.Run("OpenAI server error body", func(t *testing.T) {
		input := UpstreamInputFromResponse(
			http.StatusBadGateway,
			nil,
			[]byte(`{"error":{"type":"server_error","code":"server_error"}}`),
		)

		require.Equal(t, FailureClassUpstreamOverloaded, input.FailureClass)
	})
}

# Error Contract

Sub2API exposes immutable error identifiers in the `S2A-xxxx` form and stable
English machine codes in `SCREAMING_SNAKE_CASE`. Public messages must match this
catalog exactly and must not include upstream messages, account data, internal
addresses, or raw implementation errors.

## Response Fields

HTTP responses expose `X-Sub2API-Error-ID`, `X-Sub2API-Error-Code`,
`X-Sub2API-Retryable`, and `X-Request-ID`. Retryable errors with a known delay
also expose `Retry-After` as whole seconds.

OpenAI and Anthropic compatibility responses add `error_id`, `sub2api_code`,
`retryable`, `retry_after`, and `request_id` inside their existing error object.
Generic API responses retain legacy `code` and `reason`, and add `error_id`,
`error_code`, `retryable`, `retry_after`, and `request_id`.

## Catalog

| ID | Code | HTTP | Message |
| --- | --- | --- | --- |
| S2A-1001 | INVALID_REQUEST | 400 | The request is invalid. |
| S2A-1002 | REQUEST_BODY_INVALID | 400 | The request body is invalid. |
| S2A-1003 | REQUEST_TOO_LARGE | 413 | The request body is too large. |
| S2A-1004 | MODEL_NOT_SUPPORTED | 400 | The requested model is not supported. |
| S2A-1005 | CONTENT_POLICY_VIOLATION | 400 | The request was blocked by the content policy. |
| S2A-1006 | ENDPOINT_NOT_FOUND | 404 | The requested endpoint was not found. |
| S2A-2001 | API_KEY_REQUIRED | 401 | An API key is required. |
| S2A-2002 | API_KEY_INVALID | 401 | The API key is invalid. |
| S2A-2003 | API_KEY_INACTIVE | 403 | This API key is inactive. |
| S2A-2004 | ACCESS_DENIED | 403 | You do not have permission to perform this action. |
| S2A-2005 | SESSION_EXPIRED | 401 | Your session has expired. Please sign in again. |
| S2A-2006 | ACCOUNT_SUSPENDED | 403 | This account is unavailable. Please contact support. |
| S2A-3001 | RATE_LIMIT_EXCEEDED | 429 | The request rate limit has been exceeded. Please retry later. |
| S2A-3002 | CONCURRENCY_LIMIT_EXCEEDED | 429 | The concurrent request limit has been exceeded. Please retry later. |
| S2A-3003 | SUBSCRIPTION_QUOTA_EXCEEDED | 429 | The subscription quota has been exceeded. Please retry later. |
| S2A-3004 | BALANCE_INSUFFICIENT | 402 | Insufficient balance to process this request. |
| S2A-3005 | BILLING_RESERVATION_REJECTED | 402 | The available balance cannot authorize this request. |
| S2A-3006 | BILLING_SERVICE_UNAVAILABLE | 503 | The billing service is temporarily unavailable. Please retry later. |
| S2A-4001 | RESOURCE_NOT_FOUND | 404 | The requested resource was not found. |
| S2A-4002 | RESOURCE_CONFLICT | 409 | The requested operation conflicts with the current resource state. |
| S2A-4003 | OPERATION_NOT_ALLOWED | 409 | This operation is not allowed in the current state. |
| S2A-4004 | OPERATION_IN_PROGRESS | 409 | The requested operation is already in progress. |
| S2A-4005 | FEATURE_UNAVAILABLE | 503 | This feature is temporarily unavailable. Please retry later. |
| S2A-5001 | NO_AVAILABLE_UPSTREAM | 503 | No upstream service is currently available. Please retry later. |
| S2A-5002 | UPSTREAM_CREDENTIALS_UNAVAILABLE | 503 | No upstream credentials are currently available. Please retry later. |
| S2A-5003 | UPSTREAM_AUTHENTICATION_FAILED | 502 | The upstream service rejected its credentials. |
| S2A-5004 | UPSTREAM_RATE_LIMITED | 429 | The upstream service is rate limited. Please retry later. |
| S2A-5005 | UPSTREAM_MODEL_UNAVAILABLE | 503 | The requested model is not currently available from the upstream service. |
| S2A-5006 | UPSTREAM_OVERLOADED | 503 | The upstream service is temporarily overloaded. Please retry later. |
| S2A-5007 | UPSTREAM_TIMEOUT | 504 | The upstream service did not respond in time. Please retry later. |
| S2A-5008 | UPSTREAM_CONNECTION_FAILED | 502 | The gateway could not connect to the upstream service. |
| S2A-5009 | UPSTREAM_INVALID_RESPONSE | 502 | The upstream service returned an invalid response. |
| S2A-5010 | UPSTREAM_ACCESS_DENIED | 502 | The upstream service denied the request. |
| S2A-6001 | STREAM_INTERRUPTED | stream event | The response stream was interrupted. Please retry the request. |
| S2A-6002 | STREAM_PROTOCOL_ERROR | stream event | The response stream returned an invalid event. |
| S2A-6003 | WEBSOCKET_CONNECTION_CLOSED | WebSocket event | The WebSocket connection was closed before the request completed. |
| S2A-9001 | INTERNAL_ERROR | 500 | An internal error occurred. Please retry later. |
| S2A-9002 | PLATFORM_DEPENDENCY_UNAVAILABLE | 503 | A required platform service is temporarily unavailable. Please retry later. |

## Upstream Precedence

Sub2API trusts only the allowlisted `X-CLIProxy-Error-Class` values and matching
OpenAI-compatible `error.code` values from configured upstream accounts. An
all-account rate-limit signal has precedence over the raw proxy status and keeps
its `Retry-After`. Unknown upstream fields are diagnostic only.

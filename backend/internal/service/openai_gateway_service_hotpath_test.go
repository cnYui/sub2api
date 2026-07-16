package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAIBillingAuthorizerStub struct {
	authorization   *OpenAIBillingAuthorization
	authorizeErr    error
	authorizeCalls  int
	input           OpenAIBillingAuthorizationInput
	dispatchedIDs   []int64
	unknownIDs      []int64
	releasedIDs     []int64
	dispatchedReady bool
}

func (s *openAIBillingAuthorizerStub) Authorize(_ context.Context, input OpenAIBillingAuthorizationInput) (*OpenAIBillingAuthorization, error) {
	s.authorizeCalls++
	s.input = input
	return s.authorization, s.authorizeErr
}

func TestOpenAIGatewayService_Forward_ReusesRequestAuthorizationAcrossFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reservationID := int64(94)
	authorizer := &openAIBillingAuthorizerStub{authorization: &OpenAIBillingAuthorization{
		Source:        BillingSourceTrafficCredit,
		ReservationID: &reservationID,
		EffectiveBody: []byte(`{"model":"gpt-5-account-a","stream":false,"input":"hello","max_output_tokens":256}`),
		Enforced:      true,
	}}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusInternalServerError,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"temporary failure"}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
		},
	}}
	svc := &OpenAIGatewayService{
		cfg:                         &config.Config{},
		httpUpstream:                upstream,
		billingAuthorizationService: authorizer,
	}
	c := newOpenAIBillingAuthorizationGinContext()
	firstAccount := newOpenAIBillingAuthorizationTestAccount()
	firstAccount.ID = 71
	firstAccount.Credentials["model_mapping"] = map[string]any{"gpt-5": "gpt-5-account-a"}
	secondAccount := newOpenAIBillingAuthorizationTestAccount()
	secondAccount.ID = 72
	secondAccount.Credentials["model_mapping"] = map[string]any{"gpt-5": "gpt-5-account-b"}
	body := []byte(`{"model":"gpt-5","stream":false,"input":"hello"}`)

	first, firstErr := svc.Forward(c.Request.Context(), c, firstAccount, body)
	require.Error(t, firstErr)
	require.Nil(t, first)
	second, secondErr := svc.Forward(c.Request.Context(), c, secondAccount, body)

	require.NoError(t, secondErr)
	require.NotNil(t, second)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "gpt-5-account-b", gjson.GetBytes(upstream.bodies[1], "model").String())
	require.Equal(t, int64(256), gjson.GetBytes(upstream.bodies[1], "max_output_tokens").Int())
	require.Equal(t, HashUsageRequestPayload(body), authorizer.input.RequestFingerprint)
	require.Equal(t, 1, authorizer.authorizeCalls)
	require.Equal(t, reservationID, *second.BillingAuthorization.ReservationID)
}

func (s *openAIBillingAuthorizerStub) MarkDispatched(_ context.Context, reservationID int64) error {
	s.dispatchedIDs = append(s.dispatchedIDs, reservationID)
	s.dispatchedReady = true
	return nil
}

func (s *openAIBillingAuthorizerStub) MarkUnknown(_ context.Context, reservationID int64, _ string) error {
	s.unknownIDs = append(s.unknownIDs, reservationID)
	return nil
}

func (s *openAIBillingAuthorizerStub) Release(_ context.Context, reservationID int64) error {
	s.releasedIDs = append(s.releasedIDs, reservationID)
	return nil
}

type dispatchedOpenAIHTTPUpstream struct {
	authorizer *openAIBillingAuthorizerStub
	resp       *http.Response
	err        error
	called     bool
	lastBody   []byte
}

func (u *dispatchedOpenAIHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.called = true
	if !u.authorizer.dispatchedReady {
		return nil, errors.New("request dispatched before billing reservation")
	}
	if req != nil && req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		u.lastBody = append([]byte(nil), body...)
		_ = req.Body.Close()
		req.Body = io.NopCloser(strings.NewReader(string(body)))
	}
	return u.resp, u.err
}

func (u *dispatchedOpenAIHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestOpenAIGatewayService_Forward_AuthorizesFinalBodyBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reservationID := int64(91)
	authorizer := &openAIBillingAuthorizerStub{authorization: &OpenAIBillingAuthorization{
		Source:        BillingSourceTrafficCredit,
		ReservationID: &reservationID,
		EffectiveBody: []byte(`{"model":"gpt-5","stream":false,"reasoning":{"effort":"none"},"input":"hello","max_output_tokens":256}`),
		Enforced:      true,
	}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:                         &config.Config{},
		httpUpstream:                upstream,
		billingAuthorizationService: authorizer,
	}
	account := newOpenAIBillingAuthorizationTestAccount()
	c := newOpenAIBillingAuthorizationGinContext()

	result, err := svc.Forward(c.Request.Context(), c, account, []byte(`{"model":"gpt-5","stream":false,"reasoning":{"effort":"minimal"},"input":"hello"}`))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, reservationID, *result.BillingAuthorization.ReservationID)
	require.Equal(t, "none", gjson.GetBytes(authorizer.input.Body, "reasoning.effort").String())
	require.NotEmpty(t, gjson.GetBytes(authorizer.input.Body, "instructions").String())
	require.JSONEq(t, string(authorizer.authorization.EffectiveBody), string(upstream.lastBody))
}

func TestOpenAIGatewayService_Forward_RejectsBeforeUpstreamWhenAuthorizationFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authorizer := &openAIBillingAuthorizerStub{authorizeErr: ErrTrafficCreditInsufficient}
	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{
		cfg:                         &config.Config{},
		httpUpstream:                upstream,
		billingAuthorizationService: authorizer,
	}
	c := newOpenAIBillingAuthorizationGinContext()

	result, err := svc.Forward(c.Request.Context(), c, newOpenAIBillingAuthorizationTestAccount(), []byte(`{"model":"gpt-5","stream":false,"input":"hello"}`))

	require.ErrorIs(t, err, ErrTrafficCreditInsufficient)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq)
	require.Equal(t, http.StatusPaymentRequired, c.Writer.Status())
}

func TestOpenAIGatewayService_Forward_MarksReservationUnknownOnTransportFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reservationID := int64(92)
	authorizer := &openAIBillingAuthorizerStub{authorization: &OpenAIBillingAuthorization{
		Source:        BillingSourceTrafficCredit,
		ReservationID: &reservationID,
		EffectiveBody: []byte(`{"model":"gpt-5","stream":false,"input":"hello","max_output_tokens":256}`),
		Enforced:      true,
	}}
	upstream := &dispatchedOpenAIHTTPUpstream{authorizer: authorizer, err: errors.New("TLS result unknown")}
	svc := &OpenAIGatewayService{
		cfg:                         &config.Config{},
		httpUpstream:                upstream,
		billingAuthorizationService: authorizer,
	}
	c := newOpenAIBillingAuthorizationGinContext()

	result, err := svc.Forward(c.Request.Context(), c, newOpenAIBillingAuthorizationTestAccount(), []byte(`{"model":"gpt-5","stream":false,"input":"hello"}`))

	require.Error(t, err)
	require.Nil(t, result)
	require.True(t, upstream.called)
	require.Equal(t, []int64{reservationID}, authorizer.dispatchedIDs)
	require.Equal(t, []int64{reservationID}, authorizer.unknownIDs)
	require.Empty(t, authorizer.releasedIDs)
}

func TestOpenAIGatewayService_Forward_ReleasesReservationWhenTokenLoadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reservationID := int64(93)
	authorizer := &openAIBillingAuthorizerStub{authorization: &OpenAIBillingAuthorization{
		Source:        BillingSourceTrafficCredit,
		ReservationID: &reservationID,
		EffectiveBody: []byte(`{"model":"gpt-5","stream":false,"input":"hello","max_output_tokens":256}`),
		Enforced:      true,
	}}
	svc := &OpenAIGatewayService{
		cfg:                         &config.Config{},
		httpUpstream:                &httpUpstreamRecorder{},
		billingAuthorizationService: authorizer,
	}
	account := newOpenAIBillingAuthorizationTestAccount()
	delete(account.Credentials, "api_key")
	c := newOpenAIBillingAuthorizationGinContext()

	result, err := svc.Forward(c.Request.Context(), c, account, []byte(`{"model":"gpt-5","stream":false,"input":"hello"}`))

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, []int64{reservationID}, authorizer.releasedIDs)
	require.Empty(t, authorizer.dispatchedIDs)
}

func TestOpenAIGatewayService_RawChatCompletions_AuthorizesBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reservationID := int64(95)
	authorizer := &openAIBillingAuthorizerStub{authorization: &OpenAIBillingAuthorization{
		Source:        BillingSourceTrafficCredit,
		ReservationID: &reservationID,
		EffectiveBody: []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":false,"max_tokens":256}`),
		Enforced:      true,
	}}
	upstream := &dispatchedOpenAIHTTPUpstream{
		authorizer: authorizer,
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"raw-chat-req"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_1","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2}}`)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg:                         &config.Config{},
		httpUpstream:                upstream,
		billingAuthorizationService: authorizer,
	}
	account := newOpenAIBillingAuthorizationTestAccount()
	account.Extra = map[string]any{"responses_support": "force_chat_completions"}
	c := newOpenAIBillingAuthorizationGinContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)

	result, err := svc.forwardAsRawChatCompletions(
		c.Request.Context(),
		c,
		account,
		[]byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":false}`),
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, authorizer.authorizeCalls)
	require.Equal(t, "max_tokens", authorizer.input.OutputLimitField)
	require.Equal(t, HashUsageRequestPayload([]byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}],"stream":false}`)), authorizer.input.RequestFingerprint)
	require.Equal(t, []int64{reservationID}, authorizer.dispatchedIDs)
	require.JSONEq(t, string(authorizer.authorization.EffectiveBody), string(upstream.lastBody))
	require.Equal(t, reservationID, *result.BillingAuthorization.ReservationID)
}

func newOpenAIBillingAuthorizationTestAccount() *Account {
	return &Account{
		ID:          71,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
}

func newOpenAIBillingAuthorizationGinContext() *gin.Context {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	request := httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	request = request.WithContext(context.WithValue(request.Context(), ctxkey.RequestID, "billing-auth-req"))
	c.Request = request
	groupID := int64(2)
	c.Set("api_key", &APIKey{
		ID:      9,
		UserID:  7,
		GroupID: &groupID,
		User:    &User{ID: 7, Balance: 0},
		Group:   &Group{ID: 2, SubscriptionType: SubscriptionTypeStandard, RateMultiplier: 1},
	})
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	return c
}

func TestOpenAIRequestView_ExtractsRawScalars(t *testing.T) {
	view := newOpenAIRequestView([]byte(`{"model":" gpt-5 ","stream":true,"prompt_cache_key":" ses-1 ","previous_response_id":" resp-1 ","service_tier":" fast ","reasoning":{"effort":" medium "}}`))

	require.Equal(t, "gpt-5", view.Model)
	require.True(t, view.Stream)
	require.Equal(t, "ses-1", view.PromptCacheKey)
	require.Equal(t, "resp-1", view.PreviousResponseID)
	require.Equal(t, "fast", view.ServiceTier)
	require.Equal(t, "medium", view.ReasoningEffort)
}

func TestOpenAIRequestView_DecodeKeepsFullMapBehavior(t *testing.T) {
	view := newOpenAIRequestView([]byte(`{"model":"gpt-5","stream":true,"input":[{"type":"message","content":"hi"}]}`))

	reqBody, err := view.Decode(nil)
	require.NoError(t, err)
	require.Equal(t, "gpt-5", reqBody["model"])
	require.IsType(t, []any{}, reqBody["input"])
}

func TestOpenAIRequestView_ApplyPatches(t *testing.T) {
	view := newOpenAIRequestView([]byte(`{"model":"gpt-5","previous_response_id":"resp_1","reasoning":{"effort":"minimal"},"input":[{"type":"message","content":"hi"}]}`))
	view.MarkPatchSet("model", "gpt-5.1")
	view.MarkPatchDelete("previous_response_id")
	view.MarkPatchSet("reasoning.effort", "none")

	patched, err := view.ApplyPatches()
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5.1","reasoning":{"effort":"none"},"input":[{"type":"message","content":"hi"}]}`, string(patched))
}

func TestOpenAIRequestView_RejectsEscapedPatchPath(t *testing.T) {
	view := newOpenAIRequestView([]byte(`{"metadata":{"user.id":"old"}}`))
	view.MarkPatchSet(`metadata.user\.id`, "new")

	require.False(t, view.HasPatches())
	_, err := view.ApplyPatches()
	require.Error(t, err)
}

func TestOpenAIRequestView_ApplyPatchesDisabled(t *testing.T) {
	view := newOpenAIRequestView([]byte(`{"model":"gpt-5"}`))
	view.MarkPatchSet("model", "gpt-5.1")
	view.DisablePatches()

	_, err := view.ApplyPatches()
	require.Error(t, err)
}

func TestOpenAIRequestView_HasPatches(t *testing.T) {
	view := newOpenAIRequestView([]byte(`{"model":"gpt-5"}`))
	require.False(t, view.HasPatches())

	view.MarkPatchSet("model", "gpt-5.1")
	require.True(t, view.HasPatches())

	view.DisablePatches()
	require.False(t, view.HasPatches())
}

func TestOpenAIGatewayService_Forward_HTTPPatchPathKeepsLargeInputRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"usage":{"input_tokens":1,"output_tokens":2,"input_tokens_details":{"cached_tokens":0}}}`,
			)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          1,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-5","stream":false,"reasoning":{"effort":"minimal"},"input":[{"type":"message","content":[{"type":"input_text","text":"hi","nonce":9007199254740993}]}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	// 合成路径默认 instructions 现按模型填入真实 Codex base prompt（此处 inbound model=gpt-5）。
	encodedInstr, _ := json.Marshal(defaultCodexSynthInstructions("gpt-5"))
	expectedBody := fmt.Sprintf(`{"model":"gpt-5","stream":false,"reasoning":{"effort":"none"},"instructions":%s,"input":[{"type":"message","content":[{"type":"input_text","text":"hi","nonce":9007199254740993}]}]}`, string(encodedInstr))
	require.JSONEq(t, expectedBody, string(upstream.lastBody))
	require.Equal(t, "9007199254740993", gjson.GetBytes(upstream.lastBody, "input.0.content.0.nonce").Raw)
}

func TestOpenAIGatewayService_Forward_DecodedMutationKeepsLaterFieldDeletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          2,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-5.4","stream":false,"max_completion_tokens":12,"tools":[{"type":"image_generation","format":"png"}],"input":[{"type":"message","content":"draw"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, gjson.GetBytes(upstream.lastBody, "max_completion_tokens").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "tools.0.format").Exists())
	require.Equal(t, "png", gjson.GetBytes(upstream.lastBody, "tools.0.output_format").String())
}

func TestOpenAIGatewayService_Forward_MappedImageModelUsesImageGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          3,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":       "sk-test",
			"base_url":      "https://example.com",
			"model_mapping": map[string]any{"draw-alias": "gpt-image-2"},
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Set("api_key", &APIKey{Group: &Group{AllowImageGeneration: false}})
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"draw-alias","stream":false,"input":"draw"}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestOpenAIGatewayService_Forward_TextDataImageDoesNotForceMapMarshal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          4,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-5","stream":false,"input":[{"type":"message","content":[{"type":"input_text","text":"literal data:image/png;base64, only","nonce":1e1000000}]}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "1e1000000", gjson.GetBytes(upstream.lastBody, "input.0.content.0.nonce").Raw)
}

func TestOpenAIGatewayService_Forward_ImageToolBillingDoesNotForceFullDecode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"output":[{"id":"ig_1","type":"image_generation_call","result":"final-image"}],"usage":{"input_tokens":1,"output_tokens":2}}`,
			)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          9,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-5","stream":false,"tools":[{"type":"image_generation","model":"gpt-image-2","size":"2048x1152"}],"input":[{"type":"message","content":[{"type":"input_text","text":"draw","nonce":1e1000000}]}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "1e1000000", gjson.GetBytes(upstream.lastBody, "input.0.content.0.nonce").Raw)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "2K", result.ImageSize)
	require.Equal(t, "gpt-image-2", result.BillingModel)
	require.Equal(t, "gpt-5", result.MainBillingModel)
	require.Equal(t, "gpt-image-2", result.ImageBillingModel)
}

func TestOpenAIGatewayService_Forward_ImageToolWithImageOnlyModelIsNormalized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          11,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-image-2","stream":false,"tools":[{"type":"image_generation","model":"gpt-image-2"}],"input":"draw"}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, openAIImagesResponsesMainModel, gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestOpenAIGatewayService_Forward_HTTPRetryRecoveryDoesNotDecodeBeforeError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"invalid_encrypted_content","type":"invalid_request_error","message":"bad encrypted content"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
			},
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          10,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-5","stream":false,"input":[{"type":"reasoning","encrypted_content":"gAAA","summary":[{"type":"summary_text","text":"keep me"}]},{"type":"message","content":[{"type":"input_text","text":"hi","nonce":9007199254740993}]}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "gAAA", gjson.GetBytes(upstream.bodies[0], "input.0.encrypted_content").String())
	require.Equal(t, "9007199254740993", gjson.GetBytes(upstream.bodies[0], "input.1.content.0.nonce").Raw)
	require.False(t, gjson.GetBytes(upstream.bodies[1], "input.0.encrypted_content").Exists())
	require.Equal(t, "summary_text", gjson.GetBytes(upstream.bodies[1], "input.0.summary.0.type").String())
}

func TestOpenAIGatewayService_Forward_CodexSparkRejectsEscapedInputImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          5,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-5.3-codex-spark","stream":false,"input":[{"type":"input_` + "\\u0069" + `mage","file_id":"file_1"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpenAIGatewayService_Forward_CodexBridgeInjectionSetsImageBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"output":[{"id":"ig_1","type":"image_generation_call","result":"final-image","size":"1024x1024"}],"usage":{"input_tokens":1,"output_tokens":2}}`,
			)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Gateway.ForceCodexCLI = true
	cfg.Gateway.CodexImageGenerationBridgeEnabled = true
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          7,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Set("api_key", &APIKey{Group: &Group{AllowImageGeneration: true}})
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-5","stream":false,"input":"draw if needed"}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "2K", result.ImageSize)
	require.Equal(t, "gpt-image-2", result.BillingModel)
	require.Equal(t, "gpt-5", result.MainBillingModel)
	require.Equal(t, "gpt-image-2", result.ImageBillingModel)
}

func TestOpenAIGatewayService_Forward_HTTPDeletesPreviousResponseIDWhenPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	account := &Account{
		ID:          8,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}

	for _, body := range [][]byte{
		[]byte(`{"model":"gpt-5","stream":false,"previous_response_id":"","input":"hi"}`),
		[]byte(`{"model":"gpt-5","stream":false,"previous_response_id":null,"input":"hi"}`),
	} {
		upstream := &httpUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
			},
		}
		svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

		result, err := svc.Forward(context.Background(), c, account, body)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, gjson.GetBytes(upstream.lastBody, "previous_response_id").Exists())
	}
}

func TestOpenAIGatewayService_Forward_StripsImageGenerationToolForSparkAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          11,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	// Allow image generation so the tool is normalized (not gated out), reproducing
	// the leak the strip must override.
	c.Set("api_key", &APIKey{Group: &Group{AllowImageGeneration: true}})
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-5.3-codex-spark","stream":false,"input":"hi","tools":[{"type":"function","name":"shell"},{"type":"image_generation","output_format":"png"}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.False(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
	require.True(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="function")`).Exists())
}

func TestOpenAIRequestBodyMayContainEmptyBase64InputImageSeesEscapedJSON(t *testing.T) {
	body := []byte(`{"input":[{"type":"message","content":[{"type":"input_image","image_` + "\\u0075" + `rl":"data:image/png;base64` + "\\u002c" + `   "}]}]}`)

	require.True(t, openAIRequestBodyMayContainEmptyBase64InputImage(body))
}

func TestOpenAIRequestBodyMayContainEmptyBase64InputImageSeesEscapedImageType(t *testing.T) {
	body := []byte(`{"input":[{"type":"message","content":[{"type":"input_` + "\\u0069" + `mage","image_url":"data:image/png;base64,   "}]}]}`)

	require.True(t, openAIRequestBodyMayContainEmptyBase64InputImage(body))
}

func TestOpenAIRequestBodyMayContainEmptyBase64InputImageSeesEscapedInputPrefix(t *testing.T) {
	body := []byte(`{"input":[{"type":"message","content":[{"type":"inp` + "\\u0075" + `t_image","image_url":"data:image/png;base64,   "}]}]}`)

	require.True(t, openAIRequestBodyMayContainEmptyBase64InputImage(body))
}

func TestOpenAIGatewayService_Forward_ImageOnlyModelKeepsSupportedVerbosity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1,"output_tokens":2}}`)),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID:          6,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com",
		},
		Extra: map[string]any{"use_responses_api": true},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	body := []byte(`{"model":"gpt-image-2","stream":false,"text":{"verbosity":"low"},"input":"draw"}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "low", gjson.GetBytes(upstream.lastBody, "text.verbosity").String())
	require.Equal(t, openAIImagesResponsesMainModel, gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestExtractOpenAIRequestMetaFromBody(t *testing.T) {
	tests := []struct {
		name          string
		body          []byte
		wantModel     string
		wantStream    bool
		wantPromptKey string
	}{
		{
			name:          "完整字段",
			body:          []byte(`{"model":"gpt-5","stream":true,"prompt_cache_key":" ses-1 "}`),
			wantModel:     "gpt-5",
			wantStream:    true,
			wantPromptKey: "ses-1",
		},
		{
			name:          "缺失可选字段",
			body:          []byte(`{"model":"gpt-4"}`),
			wantModel:     "gpt-4",
			wantStream:    false,
			wantPromptKey: "",
		},
		{
			name:          "空请求体",
			body:          nil,
			wantModel:     "",
			wantStream:    false,
			wantPromptKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, stream, promptKey := extractOpenAIRequestMetaFromBody(tt.body)
			require.Equal(t, tt.wantModel, model)
			require.Equal(t, tt.wantStream, stream)
			require.Equal(t, tt.wantPromptKey, promptKey)
		})
	}
}

func TestExtractOpenAIReasoningEffortFromBody(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		model     string
		wantNil   bool
		wantValue string
	}{
		{
			name:      "优先读取 reasoning.effort",
			body:      []byte(`{"reasoning":{"effort":"medium"}}`),
			model:     "gpt-5-high",
			wantNil:   false,
			wantValue: "medium",
		},
		{
			name:      "兼容 reasoning_effort",
			body:      []byte(`{"reasoning_effort":"x-high"}`),
			model:     "",
			wantNil:   false,
			wantValue: "xhigh",
		},
		{
			name:      "DeepSeek max 归一化为 xhigh",
			body:      []byte(`{"reasoning_effort":"max"}`),
			model:     "deepseek-v4-pro",
			wantNil:   false,
			wantValue: "xhigh",
		},
		{
			name:    "minimal 归一化为空",
			body:    []byte(`{"reasoning":{"effort":"minimal"}}`),
			model:   "gpt-5-high",
			wantNil: true,
		},
		{
			name:      "缺失字段时从模型后缀推导",
			body:      []byte(`{"input":"hi"}`),
			model:     "gpt-5-high",
			wantNil:   false,
			wantValue: "high",
		},
		{
			name:    "未知后缀不返回",
			body:    []byte(`{"input":"hi"}`),
			model:   "gpt-5-unknown",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOpenAIReasoningEffortFromBody(tt.body, tt.model)
			if tt.wantNil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tt.wantValue, *got)
		})
	}
}

func TestGetOpenAIRequestBodyMap_ParseError(t *testing.T) {
	_, err := getOpenAIRequestBodyMap(nil, []byte(`{invalid-json`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse request")
}

func TestGetOpenAIRequestBodyMap_DoesNotWriteContextCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	got, err := getOpenAIRequestBodyMap(c, []byte(`{"model":"gpt-5","stream":true}`))
	require.NoError(t, err)
	require.Equal(t, "gpt-5", got["model"])
	require.Empty(t, c.Keys)
}

func TestSanitizeEmptyBase64InputImagesInOpenAIRequestBodyMap(t *testing.T) {
	var reqBody map[string]any
	require.NoError(t, json.Unmarshal([]byte(`{
		"model":"gpt-5.4",
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"Describe this"},
				{"type":"input_image","image_url":"data:image/png;base64,   "},
				{"type":"input_image","image_url":"data:image/png;base64,abc123"}
			]},
			{"role":"user","content":[
				{"type":"input_image","image_url":"data:image/png;base64,"}
			]},
			{"type":"input_image","image_url":"data:image/png;base64,"},
			{"type":"input_image","image_url":"data:image/png;base64,top-level-valid"}
		]
	}`), &reqBody))

	require.True(t, sanitizeEmptyBase64InputImagesInOpenAIRequestBodyMap(reqBody))

	normalized, err := json.Marshal(reqBody)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model":"gpt-5.4",
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"Describe this"},
				{"type":"input_image","image_url":"data:image/png;base64,abc123"}
			]},
			{"type":"input_image","image_url":"data:image/png;base64,top-level-valid"}
		]
	}`, string(normalized))
}

func TestSanitizeEmptyBase64InputImagesInOpenAIBody(t *testing.T) {
	body, changed, err := sanitizeEmptyBase64InputImagesInOpenAIBody([]byte(`{
		"model":"gpt-5.4",
		"stream":true,
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"Describe this"},
				{"type":"input_image","image_url":"data:image/png;base64,"}
			]}
		]
	}`))
	require.NoError(t, err)
	require.True(t, changed)
	require.JSONEq(t, `{
		"model":"gpt-5.4",
		"stream":true,
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"Describe this"}
			]}
		]
	}`, string(body))
}

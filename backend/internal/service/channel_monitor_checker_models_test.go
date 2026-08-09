//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func swapMonitorHTTPClientForModelsTest(t *testing.T) {
	t.Helper()
	original := monitorHTTPClient
	monitorHTTPClient = &http.Client{Timeout: 5 * time.Second}
	t.Cleanup(func() { monitorHTTPClient = original })
}

func TestRunChecksForModels_UsesSingleAuthenticatedGet(t *testing.T) {
	swapMonitorHTTPClientForModelsTest(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != providerModelsPath {
			t.Fatalf("expected GET %s, got %s %s", providerModelsPath, r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if r.ContentLength != 0 {
			t.Fatalf("models probe must not send a request body, content length=%d", r.ContentLength)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   []map[string]string{{"id": "gpt-test", "object": "model"}},
		})
	}))
	defer server.Close()

	results := runChecksForModels(context.Background(), MonitorProviderOpenAI, server.URL, "sk-test", []string{"gpt-test", "missing-model"}, &CheckOptions{APIMode: MonitorAPIModeModels})
	if len(results) != 2 {
		t.Fatalf("expected two model results, got %d", len(results))
	}
	if requests != 1 {
		t.Fatalf("expected one shared models request, got %d", requests)
	}
	if results[0].Status != MonitorStatusOperational {
		t.Fatalf("existing model should be operational, got %s (%s)", results[0].Status, results[0].Message)
	}
	if results[1].Status != MonitorStatusFailed {
		t.Fatalf("missing model should be failed, got %s (%s)", results[1].Status, results[1].Message)
	}
}

func TestRunChecksForModels_RejectsNon2xxWithoutInference(t *testing.T) {
	swapMonitorHTTPClientForModelsTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	results := runChecksForModels(context.Background(), MonitorProviderOpenAI, server.URL, "sk-test", []string{"gpt-test"}, &CheckOptions{APIMode: MonitorAPIModeModels})
	if results[0].Status != MonitorStatusError {
		t.Fatalf("non-2xx models response should be error, got %s", results[0].Status)
	}
	if results[0].Message == "" {
		t.Fatal("non-2xx response should preserve a diagnostic message")
	}
}

func TestValidateAPIModeOnlyAllowsModels(t *testing.T) {
	if err := validateAPIMode(MonitorProviderOpenAI, MonitorAPIModeModels); err != nil {
		t.Fatalf("models mode should be valid: %v", err)
	}
	if err := validateAPIMode(MonitorProviderOpenAI, MonitorAPIModeChatCompletions); err == nil {
		t.Fatal("chat_completions mode should be rejected")
	}
	if err := validateAPIMode(MonitorProviderOpenAI, MonitorAPIModeResponses); err == nil {
		t.Fatal("responses mode should be rejected")
	}
}

func TestCheckAPIModeDefaultsToModels(t *testing.T) {
	if got := checkAPIMode(nil); got != MonitorAPIModeModels {
		t.Fatalf("nil check options should default to models, got %q", got)
	}
	if got := checkAPIMode(&CheckOptions{}); got != MonitorAPIModeModels {
		t.Fatalf("empty check options should default to models, got %q", got)
	}
}

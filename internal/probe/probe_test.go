package probe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"llm-api-uptime/internal/model"
)

func TestProbeOpenAISuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("x-request-id", "req-123")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "chatcmpl-123",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Hi"}},
			},
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test", "gpt-4")

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if result.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want req-123", result.RequestID)
	}
	if result.LatencyMs <= 0 {
		t.Error("expected positive latency")
	}
}

func TestProbeOpenAIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-request-id", "req-456")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "model_not_found",
				"message": "Model not found",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test", "invalid-model")

	if result.Status != model.StatusError {
		t.Errorf("Status = %q, want error", result.Status)
	}
	if result.ErrorCode != "model_not_found" {
		t.Errorf("ErrorCode = %q, want model_not_found", result.ErrorCode)
	}
	if result.ErrorMessage != "Model not found" {
		t.Errorf("ErrorMessage = %q, want Model not found", result.ErrorMessage)
	}
	if result.RequestID != "req-456" {
		t.Errorf("RequestID = %q, want req-456", result.RequestID)
	}
}

func TestProbeOpenAIEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test", "gpt-4")

	if result.Status != model.StatusEmptyResp {
		t.Errorf("Status = %q, want empty_response", result.Status)
	}
}

func TestProbeOpenAITimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := probeOpenAI(ctx, server.URL, "test", "gpt-4")

	if result.Status != model.StatusTimeout {
		t.Errorf("Status = %q, want timeout", result.Status)
	}
}

func TestProbeOpenAINetworkError(t *testing.T) {
	result := probeOpenAI(context.Background(), "http://localhost:1", "test", "gpt-4")

	if result.Status != model.StatusError {
		t.Errorf("Status = %q, want error", result.Status)
	}
	if result.ErrorMessage == "" {
		t.Error("expected error message")
	}
}

func TestProbeAnthropicSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("missing anthropic-version header")
		}

		w.Header().Set("x-request-id", "req-789")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "msg-123",
			"content": []map[string]string{
				{"text": "Hi", "type": "text"},
			},
		})
	}))
	defer server.Close()

	result := probeAnthropic(context.Background(), server.URL, "test", "claude-3")

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.RequestID != "req-789" {
		t.Errorf("RequestID = %q, want req-789", result.RequestID)
	}
}

func TestProbeAnthropicError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "invalid_request_error",
				"message": "Invalid model",
			},
		})
	}))
	defer server.Close()

	result := probeAnthropic(context.Background(), server.URL, "test", "invalid")

	if result.Status != model.StatusError {
		t.Errorf("Status = %q, want error", result.Status)
	}
	if result.ErrorCode != "invalid_request_error" {
		t.Errorf("ErrorCode = %q, want invalid_request_error", result.ErrorCode)
	}
}

func TestFetchModelListSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or invalid Authorization header")
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{
				{"id": "gpt-4"},
				{"id": "gpt-3.5-turbo"},
				{"id": "dall-e-3"},
			},
		})
	}))
	defer server.Close()

	provider := model.Provider{
		BaseURL: server.URL,
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
	}

	models, err := FetchModelList(context.Background(), provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}
	if models[0] != "gpt-4" {
		t.Errorf("first model = %q, want gpt-4", models[0])
	}
}

func TestFetchModelListAnthropicNotSupported(t *testing.T) {
	provider := model.Provider{
		BaseURL: "https://api.anthropic.com",
		APIKey:  "test-key",
		APIType: model.APITypeAnthropic,
	}

	_, err := FetchModelList(context.Background(), provider)
	if err == nil {
		t.Error("expected error for anthropic model listing")
	}
}

func TestFetchModelListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	}))
	defer server.Close()

	provider := model.Provider{
		BaseURL: server.URL,
		APIKey:  "invalid-key",
		APIType: model.APITypeOpenAI,
	}

	_, err := FetchModelList(context.Background(), provider)
	if err == nil {
		t.Error("expected error for unauthorized request")
	}
}

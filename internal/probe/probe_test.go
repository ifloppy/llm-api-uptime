package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llm-api-uptime/internal/model"
)

// Helper to write SSE response
func writeSSEResponse(w http.ResponseWriter, chunks []string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, chunk := range chunks {
		fmt.Fprintf(w, "data: %s\n\n", chunk)
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
}

func TestProbeOpenAISuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or invalid Authorization header")
		}

		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
			`{"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.RequestID != "chatcmpl-123" {
		t.Errorf("RequestID = %q, want chatcmpl-123", result.RequestID)
	}
	if result.TotalTokens != 12 {
		t.Errorf("TotalTokens = %d, want 12", result.TotalTokens)
	}
}

func TestProbeOpenAIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "invalid-model", 10)

	if result.Status != model.StatusError {
		t.Errorf("Status = %q, want error", result.Status)
	}
}

func TestProbeOpenAIEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusError && result.Status != model.StatusEmptyContent {
		t.Errorf("Status = %q, want error or empty_content", result.Status)
	}
}

func TestProbeOpenAITimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		writeSSEResponse(w, []string{`{"id":"test","choices":[{"index":0,"delta":{"content":"Hi"}}]}`})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := probeOpenAI(ctx, server.URL, "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusTimeout && result.Status != model.StatusError {
		t.Errorf("Status = %q, want timeout or error", result.Status)
	}
}

func TestProbeOpenAINetworkError(t *testing.T) {
	result := probeOpenAI(context.Background(), "http://localhost:1", "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusError {
		t.Errorf("Status = %q, want error", result.Status)
	}
	if result.ErrorMessage == "" {
		t.Error("expected error message")
	}
}

func TestProbeOpenAIUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "invalid_api_key",
				"message": "Invalid API key",
			},
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "invalid-key", "test", "gpt-4", 10)

	if result.Status != model.StatusError {
		t.Errorf("Status = %q, want error", result.Status)
	}
}

func TestProbeOpenAIEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-empty","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":0,"total_tokens":10}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusEmptyContent {
		t.Errorf("Status = %q, want empty_content", result.Status)
	}
	if result.TotalTokens != 10 {
		t.Errorf("TotalTokens = %d, want 10", result.TotalTokens)
	}
}

func TestProbeOpenAIEmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":0,"total_tokens":10}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusEmptyContent {
		t.Errorf("Status = %q, want empty_content", result.Status)
	}
}

func TestProbeOpenAIWithTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", result.PromptTokens)
	}
	if result.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", result.CompletionTokens)
	}
	if result.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", result.TotalTokens)
	}
	// TPS may be 0 on CI where latency is 0ms
	if result.LatencyMs > 0 && result.TPS <= 0 {
		t.Error("expected positive TPS when latency > 0")
	}
}

func TestProbeAnthropicSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/messages") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing or invalid x-api-key header")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "msg-123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{"text": "Hi there!", "type": "text"},
			},
			"model": "claude-3",
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		})
	}))
	defer server.Close()

	result := probeAnthropic(context.Background(), server.URL, "test-key", "test", "claude-3", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.RequestID != "msg-123" {
		t.Errorf("RequestID = %q, want msg-123", result.RequestID)
	}
	if result.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", result.PromptTokens)
	}
	if result.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", result.CompletionTokens)
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

	result := probeAnthropic(context.Background(), server.URL, "test-key", "test", "invalid", 10)

	if result.Status != model.StatusError {
		t.Errorf("Status = %q, want error", result.Status)
	}
}

func TestFetchModelListSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or invalid Authorization header")
		}

		w.Header().Set("Content-Type", "application/json")
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

// -- Provider cheating / empty content bypass tests --

func TestProbeOpenAIEmptyChoicesNoUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-empty","object":"chat.completion.chunk","choices":[]}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusEmptyContent {
		t.Errorf("Status = %q, want empty_content", result.Status)
	}
}

func TestProbeOpenAIEmptyContentNoUsageNoReasoning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-empty2","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"   "},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":0,"total_tokens":5}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusEmptyContent {
		t.Errorf("whitespace-only content should be StatusEmptyContent, got %q", result.Status)
	}
}

func TestProbeOpenAIHTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("<html><body>Service Unavailable</body></html>"))
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusError {
		t.Errorf("Status = %q, want error", result.Status)
	}
}

func TestProbeAnthropicEmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "msg-empty",
			"type":    "message",
			"role":    "assistant",
			"content": []interface{}{},
			"model":   "claude-3",
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 0,
			},
		})
	}))
	defer server.Close()

	result := probeAnthropic(context.Background(), server.URL, "test-key", "test", "claude-3", 10)

	if result.Status != model.StatusEmptyContent {
		t.Errorf("Status = %q, want empty_content", result.Status)
	}
}

func TestProbeAnthropicWithTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "msg-123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{"text": "Hello!", "type": "text"},
			},
			"model": "claude-3",
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		})
	}))
	defer server.Close()

	result := probeAnthropic(context.Background(), server.URL, "test-key", "test", "claude-3", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", result.PromptTokens)
	}
	if result.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", result.CompletionTokens)
	}
	if result.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", result.TotalTokens)
	}
	// TPS may be 0 on CI where latency is 0ms
	if result.LatencyMs > 0 && result.TPS <= 0 {
		t.Errorf("TPS should be positive, got %f", result.TPS)
	}
}

func TestProbeOpenAIEmptyContentWithReasoning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The SDK doesn't expose reasoning_content directly, so this test
		// verifies that empty content with usage is treated as empty_content
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-456","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":15,"total_tokens":25}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 64)

	if result.Status != model.StatusEmptyContent {
		t.Errorf("Status = %q, want empty_content", result.Status)
	}
}

func TestProbeOpenAIRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "rate_limit_exceeded",
				"message": "Rate limit exceeded",
				"type":    "rate_limit_error",
			},
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 10)

	if result.Status != model.StatusError {
		t.Errorf("Status = %q, want error", result.Status)
	}
}

func TestProbeOpenAISSEMultiChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-789","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
			`{"id":"chatcmpl-789","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`{"id":"chatcmpl-789","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`{"id":"chatcmpl-789","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "test", "gpt-4", 64)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.TotalTokens != 12 {
		t.Errorf("TotalTokens = %d, want 12", result.TotalTokens)
	}
}

func TestFetchModelListWithBOM(t *testing.T) {
	// The OpenAI SDK doesn't handle BOM correctly, so this test verifies
	// that BOM-prefixed responses cause an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Write BOM + JSON
		w.Write([]byte{0xEF, 0xBB, 0xBF})
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{
				{"id": "gpt-4"},
				{"id": "gpt-3.5-turbo"},
			},
		})
	}))
	defer server.Close()

	provider := model.Provider{
		BaseURL: server.URL,
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
	}

	_, err := FetchModelList(context.Background(), provider)
	// SDK doesn't handle BOM, so we expect an error
	if err == nil {
		t.Error("expected error for BOM-prefixed response")
	}
}

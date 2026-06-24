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

func TestCompatOneAPIFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-oneapi-request-id", "oneapi-req-123")
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-oneapi","object":"chat.completion.chunk","choices":[]}`,
			`{"id":"chatcmpl-oneapi","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "oneapi-test", "gpt-4", 10)

	if result.Status != model.StatusEmptyContent {
		t.Errorf("Status = %q, want empty_content", result.Status)
	}
	if result.RequestID != "oneapi-req-123" {
		t.Errorf("RequestID = %q, want oneapi-req-123", result.RequestID)
	}
	if result.TotalTokens != 30 {
		t.Errorf("TotalTokens = %d, want 30", result.TotalTokens)
	}
}

func TestCompatOneAPIWithContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-oneapi-request-id", "oneapi-req-456")
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-oneapi2","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
			`{"id":"chatcmpl-oneapi2","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello from OneAPI"},"finish_reason":null}]}`,
			`{"id":"chatcmpl-oneapi2","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":15,"completion_tokens":5,"total_tokens":20}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "oneapi-test", "gpt-4", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.RequestID != "oneapi-req-456" {
		t.Errorf("RequestID = %q, want oneapi-req-456", result.RequestID)
	}
	if result.TotalTokens != 20 {
		t.Errorf("TotalTokens = %d, want 20", result.TotalTokens)
	}
	if result.LatencyMs <= 0 {
		t.Error("expected positive latency")
	}
}

func TestCompatAnthropicProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/messages") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "proxy-key" {
			t.Errorf("missing or invalid x-api-key header")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "msg-proxy-789",
			"type": "message",
			"role": "assistant",
			"content": []map[string]interface{}{
				{"text": "Response from proxy", "type": "text"},
			},
			"model": "claude-3",
			"usage": map[string]interface{}{
				"input_tokens":  8,
				"output_tokens": 4,
			},
		})
	}))
	defer server.Close()

	result := probeAnthropic(context.Background(), server.URL, "proxy-key", "anthropic-proxy", "claude-3", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.RequestID != "msg-proxy-789" {
		t.Errorf("RequestID = %q, want msg-proxy-789", result.RequestID)
	}
	if result.PromptTokens != 8 {
		t.Errorf("PromptTokens = %d, want 8", result.PromptTokens)
	}
	if result.CompletionTokens != 4 {
		t.Errorf("CompletionTokens = %d, want 4", result.CompletionTokens)
	}
	if result.TotalTokens != 12 {
		t.Errorf("TotalTokens = %d, want 12", result.TotalTokens)
	}
}

func TestCompatDifferentBaseURLFormats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-url","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`,
		})
	}))
	defer server.Close()

	tests := []struct {
		name    string
		baseURL string
	}{
		{"no trailing slash", server.URL},
		{"trailing slash", server.URL + "/"},
		{"with v1 prefix", server.URL + "/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := probeOpenAI(context.Background(), tt.baseURL, "test-key", "url-test", "gpt-4", 10)

			if result.Status != model.StatusSuccess {
				t.Errorf("Status = %q, want success", result.Status)
			}
			if result.TotalTokens != 6 {
				t.Errorf("TotalTokens = %d, want 6", result.TotalTokens)
			}
		})
	}
}

func TestCompatCustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "nginx/1.21.6")
		w.Header().Set("X-Custom-Header", "custom-value-123")
		w.Header().Set("X-Request-Cost", "0.0003")
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-custom","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "custom-headers", "gpt-4", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", result.TotalTokens)
	}
	if result.LatencyMs <= 0 {
		t.Error("expected positive latency")
	}
}

func TestCompatHTTP2Response(t *testing.T) {
	// HTTP/2 is transparent at the application layer — SSE format is identical
	// regardless of transport version. We verify the probe handles responses
	// with HTTP/2 typical headers (h2 alt-svc, strict transport security) that
	// proxies often inject. The probe cannot accept self-signed certs because
	// probeOpenAI creates its own SDK client internally.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Alt-Svc", `h2=":443"; ma=86400`)
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-h2","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"HTTP2 response"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":11}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "h2-test", "gpt-4", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.TotalTokens != 11 {
		t.Errorf("TotalTokens = %d, want 11", result.TotalTokens)
	}
}

func TestCompatSlowResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-slow","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Slow response"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := probeOpenAI(ctx, server.URL, "test-key", "slow-test", "gpt-4", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.LatencyMs < 150 {
		t.Errorf("LatencyMs = %d, expected >= 150ms", result.LatencyMs)
	}
}

func TestCompatVeryLargeResponse(t *testing.T) {
	largeContent := strings.Repeat("This is a long response with lots of content. ", 50)
	largeChunk := fmt.Sprintf(
		`{"id":"chatcmpl-large","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":%q},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":500,"total_tokens":510}}`,
		largeContent,
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{largeChunk})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "large-test", "gpt-4", 500)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.CompletionTokens != 500 {
		t.Errorf("CompletionTokens = %d, want 500", result.CompletionTokens)
	}
	if result.TotalTokens != 510 {
		t.Errorf("TotalTokens = %d, want 510", result.TotalTokens)
	}
}

func TestCompatUnicodeContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"CJK characters", "你好世界 こんにちは 안녕하세요"},
		{"emoji", "Hello 🌍🚀✨🎉"},
		{"mixed special", "Café résumé naïve über Straße"},
		{"math symbols", "∑∏∫√∞≈≠±×÷"},
		{"full width", "ＨｅｌｌｏＷｏｒｌｄ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := fmt.Sprintf(
				`{"id":"chatcmpl-unicode","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":%q},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
				tt.content,
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeSSEResponse(w, []string{chunk})
			}))
			defer server.Close()

			result := probeOpenAI(context.Background(), server.URL, "test-key", "unicode-test", "gpt-4", 10)

			if result.Status != model.StatusSuccess {
				t.Errorf("Status = %q, want success", result.Status)
			}
			if result.TotalTokens != 15 {
				t.Errorf("TotalTokens = %d, want 15", result.TotalTokens)
			}
		})
	}
}

func TestCompatEmptyUsageField(t *testing.T) {
	tests := []struct {
		name  string
		chunk string
	}{
		{
			"no usage field",
			`{"id":"chatcmpl-nousage","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}]}`,
		},
		{
			"zero usage",
			`{"id":"chatcmpl-zerousage","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeSSEResponse(w, []string{tt.chunk})
			}))
			defer server.Close()

			result := probeOpenAI(context.Background(), server.URL, "test-key", "empty-usage", "gpt-4", 10)

			if result.Status != model.StatusSuccess {
				t.Errorf("Status = %q, want success", result.Status)
			}
			if result.TPS != 0 {
				t.Errorf("TPS = %f, want 0 when usage is zero/missing", result.TPS)
			}
		})
	}
}

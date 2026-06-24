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

// TestRedTeamFakeTokenCount tests a proxy that claims high completion_tokens
// but returns only minimal content ("Hi"). The probe should record the token
// count as-is since it trusts the API's usage field.
func TestRedTeamFakeTokenCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-fake","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5000,"total_tokens":5010}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.CompletionTokens != 5000 {
		t.Errorf("CompletionTokens = %d, want 5000 (probe trusts API-reported count)", result.CompletionTokens)
	}
	if result.TotalTokens != 5010 {
		t.Errorf("TotalTokens = %d, want 5010", result.TotalTokens)
	}
}

// TestRedTeamContentPadding tests a proxy that returns only whitespace or
// invisible characters that pass TrimSpace. All variants should be detected
// as StatusEmptyContent.
func TestRedTeamContentPadding(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"spaces", "   "},
		{"tabs", "\t\t\t"},
		{"newlines", "\n\n\n"},
		{"mixed_whitespace", " \t\n \t\n "},
		{"zero_width_chars", "\u200b\u200b\u200b"},
		{"non_breaking_spaces", "\u00a0\u00a0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Escape the content for JSON embedding
			escaped := mustEscapeJSON(tc.content)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeSSEResponse(w, []string{
					`{"id":"chatcmpl-pad","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":` + escaped + `},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":0,"total_tokens":5}}`,
				})
			}))
			defer server.Close()

			result := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)

			if result.Status != model.StatusEmptyContent {
				t.Errorf("Status = %q, want empty_content for %q", result.Status, tc.name)
			}
		})
	}
}

// TestRedTeamSuspiciouslyFastResponse tests a response returned in <10ms,
// which is likely cached. The probe accepts it as success — latency is
// recorded but no special flag is raised.
func TestRedTeamSuspiciouslyFastResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-cached","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Cached response"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.LatencyMs < 0 {
		t.Errorf("LatencyMs = %d, want non-negative", result.LatencyMs)
	}
}

// TestRedTeamFakeRequestID tests a proxy that returns a custom x-request-id
// header. The probe should prefer the header value over the stream's id field.
func TestRedTeamFakeRequestID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-request-id", "fake-request-id-12345")
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-real","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success", result.Status)
	}
	if result.RequestID != "fake-request-id-12345" {
		t.Errorf("RequestID = %q, want fake-request-id-12345 (header takes precedence)", result.RequestID)
	}
}

// TestRedTeamPartialStream tests a proxy that sends content chunks but never
// sends a [DONE] marker or usage data. The probe uses a 2s timeout to detect
// the stalled stream and should return whatever content was accumulated.
func TestRedTeamPartialStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Send content but never send [DONE] or usage
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter does not support flushing")
			return
		}
		w.Write([]byte("data: {\"id\":\"chatcmpl-partial\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Partial\"},\"finish_reason\":null}]}\n\n"))
		flusher.Flush()
		// Keep connection open — no [DONE]
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := probeOpenAI(ctx, server.URL, "test-key", "redteam", "gpt-4", 10)

	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success (partial content should be accepted)", result.Status)
	}
}

// TestRedTeamHiddenRateLimit tests a proxy that returns 200 OK with empty
// choices and zero usage — a soft rate limit disguised as a normal response.
func TestRedTeamHiddenRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-ratelimit","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)

	if result.Status != model.StatusEmptyContent {
		t.Errorf("Status = %q, want empty_content", result.Status)
	}
	if result.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d, want 0", result.TotalTokens)
	}
}

// TestRedTeamModelSubstitution tests a proxy where the request asks for gpt-4
// but the response uses gpt-3.5-turbo. The probe accepts it as success since
// it does not validate the response model against the request model.
func TestRedTeamModelSubstitution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-sub","object":"chat.completion.chunk","model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)

	// Probe accepts whatever model the response claims — no substitution detection
	if result.Status != model.StatusSuccess {
		t.Errorf("Status = %q, want success (model substitution is not detected)", result.Status)
	}
}

// TestRedTeamStatusCodeDeception tests a proxy that returns HTTP 200 but with
// a non-streaming error JSON body instead of SSE data. The probe should detect
// this as an error since the SSE parsing fails.
func TestRedTeamStatusCodeDeception(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "internal_error",
				"message": "Something went wrong",
				"type":    "server_error",
			},
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)

	// Non-SSE body causes parse failure, resulting in an error
	if result.Status != model.StatusError && result.Status != model.StatusEmptyContent {
		t.Errorf("Status = %q, want error or empty_content (deception detected via parse failure)", result.Status)
	}
}

// TestRedTeamOnlyReasoningContent tests a proxy that returns empty content
// but has usage tokens (e.g., all tokens went to reasoning_content which
// the SDK does not expose). The probe should detect this as empty_content.
func TestRedTeamOnlyReasoningContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEResponse(w, []string{
			`{"id":"chatcmpl-reasoning","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":50,"total_tokens":60}}`,
		})
	}))
	defer server.Close()

	result := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)

	if result.Status != model.StatusEmptyContent {
		t.Errorf("Status = %q, want empty_content (empty content with usage detected)", result.Status)
	}
	if result.CompletionTokens != 50 {
		t.Errorf("CompletionTokens = %d, want 50", result.CompletionTokens)
	}
}

// TestRedTeamInconsistentResponses tests a proxy that alternates between
// returning a valid response and returning empty content. Each call should
// be evaluated independently.
func TestRedTeamInconsistentResponses(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount%2 == 1 {
			writeSSEResponse(w, []string{
				`{"id":"chatcmpl-alt","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`,
			})
		} else {
			writeSSEResponse(w, []string{
				`{"id":"chatcmpl-alt","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":0,"total_tokens":5}}`,
			})
		}
	}))
	defer server.Close()

	// First call — success
	result1 := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)
	if result1.Status != model.StatusSuccess {
		t.Errorf("call 1: Status = %q, want success", result1.Status)
	}
	if result1.TotalTokens != 6 {
		t.Errorf("call 1: TotalTokens = %d, want 6", result1.TotalTokens)
	}

	// Second call — empty content
	result2 := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)
	if result2.Status != model.StatusEmptyContent {
		t.Errorf("call 2: Status = %q, want empty_content", result2.Status)
	}

	// Third call — success again
	result3 := probeOpenAI(context.Background(), server.URL, "test-key", "redteam", "gpt-4", 10)
	if result3.Status != model.StatusSuccess {
		t.Errorf("call 3: Status = %q, want success", result3.Status)
	}
}

// mustEscapeJSON wraps a string in JSON-escaped form for embedding in JSON templates.
func mustEscapeJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic("failed to escape JSON string: " + err.Error())
	}
	return string(b)
}

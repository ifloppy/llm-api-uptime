package probe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-api-uptime/internal/model"
)

type openAIRequest struct {
	Model    string           `json:"model"`
	Messages []openAIMessage  `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
	Stream   bool             `json:"stream"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChoice struct {
	Index   int `json:"index"`
	Message struct {
		Content          string `json:"content"`
		ReasoningContent string `json:"reasoning_content"`
	} `json:"message"`
	Delta *struct {
		Content string `json:"content"`
	} `json:"delta,omitempty"`
	FinishReason *string `json:"finish_reason"`
}

type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []openAIChoice `json:"choices,omitempty"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}


func probeOpenAI(ctx context.Context, baseURL, apiKey, providerName, modelID string, maxTokens int) *model.Result {
	start := time.Now()

	if maxTokens <= 0 {
		maxTokens = 2
	}

	reqBody := openAIRequest{
		Model: modelID,
		Messages: []openAIMessage{
			{Role: "user", Content: "Hi"},
		},
		MaxTokens: maxTokens,
		Stream:    false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return &model.Result{
			Status:       model.StatusError,
			ErrorMessage: fmt.Sprintf("marshal request: %v", err),
		}
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return &model.Result{
			Status:       model.StatusError,
			ErrorMessage: fmt.Sprintf("create request: %v", err),
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return &model.Result{
				Status:    model.StatusTimeout,
				LatencyMs: latency,
			}
		}
		return &model.Result{
			Status:       model.StatusError,
			LatencyMs:    latency,
			ErrorMessage: err.Error(),
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &model.Result{
			Status:       model.StatusError,
			StatusCode:   resp.StatusCode,
			LatencyMs:    latency,
			ErrorMessage: fmt.Sprintf("read response: %v", err),
		}
	}

	if len(respBody) == 0 {
		return &model.Result{
			Status:     model.StatusEmptyResp,
			StatusCode: resp.StatusCode,
			LatencyMs:  latency,
		}
	}

	// Handle SSE format: some proxies return text/event-stream even with stream=false
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		respBody = parseSSEToJSON(respBody)
	}

	respBody = stripBOM(respBody)

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return &model.Result{
			Status:       model.StatusError,
			StatusCode:   resp.StatusCode,
			LatencyMs:    latency,
			ErrorMessage: fmt.Sprintf("parse response: %v", err),
			RawError:     string(respBody),
		}
	}

	// Map SSE delta format to message: SSE uses "delta" instead of "message"
	for i := range openAIResp.Choices {
		if openAIResp.Choices[i].Message.Content == "" && openAIResp.Choices[i].Delta != nil {
			openAIResp.Choices[i].Message.Content = openAIResp.Choices[i].Delta.Content
		}
	}

	requestID := openAIResp.ID
	if rid := resp.Header.Get("x-request-id"); rid != "" {
		requestID = rid
	}
	if rid := resp.Header.Get("x-oneapi-request-id"); rid != "" {
		requestID = rid
	}

	if openAIResp.Error != nil {
		return &model.Result{
			Status:       model.StatusError,
			StatusCode:   resp.StatusCode,
			LatencyMs:    latency,
			ErrorCode:    openAIResp.Error.Code,
			ErrorMessage: openAIResp.Error.Message,
			RequestID:    requestID,
			RawError:     string(respBody),
		}
	}

	if resp.StatusCode >= 400 {
		return &model.Result{
			Status:     model.StatusError,
			StatusCode: resp.StatusCode,
			LatencyMs:  latency,
			RequestID:  requestID,
			RawError:   string(respBody),
		}
	}

	if len(openAIResp.Choices) == 0 {
		// Empty choices is expected in SSE final chunk.
		// If we have usage data, the API processed the request successfully.
		if openAIResp.Usage.TotalTokens > 0 {
			return &model.Result{
				Status:           model.StatusSuccess,
				StatusCode:       resp.StatusCode,
				LatencyMs:        latency,
				PromptTokens:     openAIResp.Usage.PromptTokens,
				CompletionTokens: openAIResp.Usage.CompletionTokens,
				TotalTokens:      openAIResp.Usage.TotalTokens,
				RequestID:        requestID,
			}
		}
		return &model.Result{
			Status:       model.StatusEmptyContent,
			StatusCode:   resp.StatusCode,
			LatencyMs:    latency,
			ErrorMessage: "empty choices in response",
			RequestID:    requestID,
			RawError:     string(respBody),
		}
	}

	content := strings.TrimSpace(openAIResp.Choices[0].Message.Content)
	reasoning := strings.TrimSpace(openAIResp.Choices[0].Message.ReasoningContent)
	if content == "" && reasoning == "" {
		return &model.Result{
			Status:       model.StatusEmptyContent,
			StatusCode:   resp.StatusCode,
			LatencyMs:    latency,
			ErrorMessage: "empty content in response",
			RequestID:    requestID,
			RawError:     string(respBody),
		}
	}

	tps := 0.0
	if latency > 0 && openAIResp.Usage.CompletionTokens > 0 {
		tps = float64(openAIResp.Usage.CompletionTokens) / (float64(latency) / 1000.0)
	}

	return &model.Result{
		Status:           model.StatusSuccess,
		StatusCode:       resp.StatusCode,
		LatencyMs:        latency,
		PromptTokens:     openAIResp.Usage.PromptTokens,
		CompletionTokens: openAIResp.Usage.CompletionTokens,
		TotalTokens:      openAIResp.Usage.TotalTokens,
		TPS:              tps,
		RequestID:        requestID,
	}
}

// parseSSEToJSON merges all SSE data chunks into a single JSON response.
// SSE format: multiple "data: {...}" lines, each is a partial JSON chunk.
// We merge delta content and keep the latest usage/finish info.
func parseSSEToJSON(body []byte) []byte {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	
	// Accumulated content from all delta chunks
	var fullContent strings.Builder
	
	// Track if we found any content delta
	hasContent := false
	
	// The last complete chunk for non-content fields (usage, id, model, etc.)
	var lastFullChunk map[string]interface{}
	
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			continue
		}
		
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		
		lastFullChunk = chunk
		
		// Extract delta content from choices
		if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if content, ok := delta["content"].(string); ok && content != "" {
						fullContent.WriteString(content)
						hasContent = true
					}
				}
			}
		}
	}
	
	if !hasContent {
		// No content accumulated, return the last chunk as-is
		if lastFullChunk != nil {
			jsonBytes, _ := json.Marshal(lastFullChunk)
			return jsonBytes
		}
		return body
	}
	
	// Build merged response: take the last chunk and inject the accumulated content
	if lastFullChunk != nil {
		if choices, ok := lastFullChunk["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				message := map[string]interface{}{
					"role":    "assistant",
					"content": fullContent.String(),
				}
				// Remove delta, add message
				delete(choice, "delta")
				choice["message"] = message
				lastFullChunk["choices"] = []interface{}{choice}
			}
		} else {
			// choices is empty in the last chunk, create a proper choice
			lastFullChunk["choices"] = []interface{}{
				map[string]interface{}{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": fullContent.String(),
					},
					"finish_reason": "stop",
				},
			}
		}
		jsonBytes, _ := json.Marshal(lastFullChunk)
		return jsonBytes
	}
	
	return body
}

package probe

import (
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

type openAIResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content           string `json:"content"`
			ReasoningContent  string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices,omitempty"`
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

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

type anthropicRequest struct {
	Model      string              `json:"model"`
	MaxTokens  int                 `json:"max_tokens"`
	Messages   []anthropicMessage  `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID    string `json:"id"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func probeAnthropic(ctx context.Context, baseURL, providerName, modelID string) *model.Result {
	start := time.Now()

	reqBody := anthropicRequest{
		Model:     modelID,
		MaxTokens: 1,
		Messages: []anthropicMessage{
			{Role: "user", Content: "Hi"},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return &model.Result{
			Status:       model.StatusError,
			ErrorMessage: fmt.Sprintf("marshal request: %v", err),
		}
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return &model.Result{
			Status:       model.StatusError,
			ErrorMessage: fmt.Sprintf("create request: %v", err),
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

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

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return &model.Result{
			Status:       model.StatusError,
			StatusCode:   resp.StatusCode,
			LatencyMs:    latency,
			ErrorMessage: fmt.Sprintf("parse response: %v", err),
			RawError:     string(respBody),
		}
	}

	requestID := anthropicResp.ID
	if rid := resp.Header.Get("x-request-id"); rid != "" {
		requestID = rid
	}

	if anthropicResp.Error != nil {
		return &model.Result{
			Status:       model.StatusError,
			StatusCode:   resp.StatusCode,
			LatencyMs:    latency,
			ErrorCode:    anthropicResp.Error.Type,
			ErrorMessage: anthropicResp.Error.Message,
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

	return &model.Result{
		Status:     model.StatusSuccess,
		StatusCode: resp.StatusCode,
		LatencyMs:  latency,
		RequestID:  requestID,
	}
}

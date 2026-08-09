package probe

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"llm-api-uptime/internal/model"
)

func probeOpenAI(ctx context.Context, baseURL, apiKey, providerName, modelID string, maxTokens int) *model.Result {
	start := time.Now()

	if maxTokens <= 0 {
		maxTokens = 2
	}

	var requestIDFromHeader string

	client := openai.NewClient(
		option.WithBaseURL(strings.TrimRight(baseURL, "/") + "/v1"),
		option.WithAPIKey(apiKey),
		option.WithMiddleware(func(req *http.Request, nxt option.MiddlewareNext) (*http.Response, error) {
			// Log request for debugging
			slog.Debug("probe request",
				"provider", providerName,
				"model", modelID,
				"method", req.Method,
				"url", req.URL.String(),
			)

			resp, err := nxt(req)
			if err == nil {
				if rid := resp.Header.Get("x-request-id"); rid != "" {
					requestIDFromHeader = rid
				} else if rid := resp.Header.Get("x-oneapi-request-id"); rid != "" {
					requestIDFromHeader = rid
				}
			}
			return resp, err
		}),
	)

	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: modelID,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hi"),
		},
		MaxTokens: openai.Int(int64(maxTokens)),
	})

	var ttftMs int
	firstChunk := true
	acc := openai.ChatCompletionAccumulator{}
	var lastUsagePromptTokens, lastUsageCompletionTokens, lastUsageTotalTokens int

	for stream.Next() {
		chunk := stream.Current()
		if firstChunk && len(chunk.Choices) > 0 {
			ttftMs = int(time.Since(start).Milliseconds())
			firstChunk = false
		}
		acc.AddChunk(chunk)

		// Fallback: manually track usage from chunk-level data
		if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			lastUsagePromptTokens = int(chunk.Usage.PromptTokens)
			lastUsageCompletionTokens = int(chunk.Usage.CompletionTokens)
			lastUsageTotalTokens = int(chunk.Usage.TotalTokens)
		}
	}

	latency := int(time.Since(start).Milliseconds())

	// Use SDK accumulated usage, or fallback to manually tracked usage
	promptTokens := int(acc.Usage.PromptTokens)
	completionTokens := int(acc.Usage.CompletionTokens)
	totalTokens := int(acc.Usage.TotalTokens)
	if promptTokens == 0 && lastUsagePromptTokens > 0 {
		promptTokens = lastUsagePromptTokens
		completionTokens = lastUsageCompletionTokens
		totalTokens = lastUsageTotalTokens
	}

	if err := stream.Err(); err != nil {
		errMsg := err.Error()
		
		slog.Debug("probe stream error",
			"provider", providerName,
			"model", modelID,
			"error", errMsg,
			"latency_ms", latency,
		)

		if ctx.Err() == context.DeadlineExceeded {
			return &model.Result{
				Status:    model.StatusTimeout,
				LatencyMs: latency,
				TTFTMs:    ttftMs,
			}
		}

		return &model.Result{
			Status:       model.StatusError,
			LatencyMs:    latency,
			TTFTMs:       ttftMs,
			ErrorMessage: errMsg,
		}
	}

	requestID := requestIDFromHeader
	if requestID == "" {
		requestID = acc.ID
	}
	if len(acc.Choices) == 0 {
		return &model.Result{
			Status:           model.StatusEmptyContent,
			StatusCode:       200,
			LatencyMs:        latency,
			TTFTMs:           ttftMs,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
			ErrorMessage:     "empty choices in response",
			RequestID:        requestID,
		}
	}

	content := strings.TrimSpace(acc.Choices[0].Message.Content)

	if !isContentMeaningful(content) {
		return &model.Result{
			Status:           model.StatusEmptyContent,
			StatusCode:       200,
			LatencyMs:        latency,
			TTFTMs:           ttftMs,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
			ErrorMessage:     "empty content in response",
			RequestID:        requestID,
		}
	}

	tps := 0.0
	tpsExcludeTTFT := 0.0
	if latency > 0 && completionTokens > 0 {
		tps = float64(completionTokens) / (float64(latency) / 1000.0)
	}
	if ttftMs > 0 && latency > ttftMs && completionTokens > 0 {
		tpsExcludeTTFT = float64(completionTokens) / (float64(latency-ttftMs) / 1000.0)
	}

	if latency < minReasonableLatencyMs {
		slog.Warn("probe succeeded unusually fast, possibly cached",
			"provider", providerName,
			"model", modelID,
			"latency_ms", latency,
		)
	}

	if completionTokens > 0 && len(content) < 10 {
		slog.Warn("short content with non-zero token count, possibly inflated",
			"provider", providerName,
			"model", modelID,
			"content_len", len(content),
			"completion_tokens", completionTokens,
		)
	}

	if softStatus, code, matched := classifyContent(content); softStatus != "" {
		slog.Warn("probe soft-fail: response body looks like a known failure template",
			"provider", providerName,
			"model", modelID,
			"code", code,
			"matched", matched,
		)
		return &model.Result{
			Status:           softStatus,
			StatusCode:       200,
			LatencyMs:        latency,
			TTFTMs:           ttftMs,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
			ErrorCode:        code,
			ErrorMessage:     fmt.Sprintf("soft-fail content matched %q: %s", code, matched),
			RequestID:        requestID,
		}
	}

	return &model.Result{
		Status:           model.StatusSuccess,
		StatusCode:       200,
		LatencyMs:        latency,
		TTFTMs:           ttftMs,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		TPS:              tps,
		TPSExcludeTTFT:   tpsExcludeTTFT,
		RequestID:        requestID,
	}
}

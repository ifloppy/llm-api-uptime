package probe

import (
	"context"
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

	acc := openai.ChatCompletionAccumulator{}
	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)
	}

	latency := int(time.Since(start).Milliseconds())

	if err := stream.Err(); err != nil {
		errMsg := err.Error()
		
		if ctx.Err() == context.DeadlineExceeded {
			return &model.Result{
				Status:    model.StatusTimeout,
				LatencyMs: latency,
			}
		}

		return &model.Result{
			Status:       model.StatusError,
			LatencyMs:    latency,
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
			PromptTokens:     int(acc.Usage.PromptTokens),
			CompletionTokens: int(acc.Usage.CompletionTokens),
			TotalTokens:      int(acc.Usage.TotalTokens),
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
			PromptTokens:     int(acc.Usage.PromptTokens),
			CompletionTokens: int(acc.Usage.CompletionTokens),
			TotalTokens:      int(acc.Usage.TotalTokens),
			ErrorMessage:     "empty content in response",
			RequestID:        requestID,
		}
	}

	tps := 0.0
	completionTokens := int(acc.Usage.CompletionTokens)
	if latency > 0 && completionTokens > 0 {
		tps = float64(completionTokens) / (float64(latency) / 1000.0)
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

	return &model.Result{
		Status:           model.StatusSuccess,
		StatusCode:       200,
		LatencyMs:        latency,
		PromptTokens:     int(acc.Usage.PromptTokens),
		CompletionTokens: completionTokens,
		TotalTokens:      int(acc.Usage.TotalTokens),
		TPS:              tps,
		RequestID:        requestID,
	}
}

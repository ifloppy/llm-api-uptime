package probe

import (
	"context"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"llm-api-uptime/internal/model"
)

func probeAnthropic(ctx context.Context, baseURL, apiKey, providerName, modelID string, maxTokens int) *model.Result {
	start := time.Now()

	if maxTokens <= 0 {
		maxTokens = 2
	}

	client := anthropic.NewClient(
		option.WithBaseURL(strings.TrimRight(baseURL, "/") + "/v1"),
		option.WithAPIKey(apiKey),
	)

	message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     modelID,
		MaxTokens: int64(maxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hi")),
		},
	})

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

	requestID := message.ID

	if len(message.Content) == 0 {
		return &model.Result{
			Status:           model.StatusEmptyContent,
			StatusCode:       200,
			LatencyMs:        latency,
			PromptTokens:     int(message.Usage.InputTokens),
			CompletionTokens: int(message.Usage.OutputTokens),
			TotalTokens:      int(message.Usage.InputTokens + message.Usage.OutputTokens),
			ErrorMessage:     "empty content in response",
			RequestID:        requestID,
		}
	}

	content := ""
	for _, block := range message.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}
	content = strings.TrimSpace(content)

	if content == "" {
		return &model.Result{
			Status:           model.StatusEmptyContent,
			StatusCode:       200,
			LatencyMs:        latency,
			PromptTokens:     int(message.Usage.InputTokens),
			CompletionTokens: int(message.Usage.OutputTokens),
			TotalTokens:      int(message.Usage.InputTokens + message.Usage.OutputTokens),
			ErrorMessage:     "empty content text in response",
			RequestID:        requestID,
		}
	}

	promptTokens := int(message.Usage.InputTokens)
	completionTokens := int(message.Usage.OutputTokens)
	totalTokens := promptTokens + completionTokens

	tps := 0.0
	if latency > 0 && completionTokens > 0 {
		tps = float64(completionTokens) / (float64(latency) / 1000.0)
	}

	return &model.Result{
		Status:           model.StatusSuccess,
		StatusCode:       200,
		LatencyMs:        latency,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		TPS:              tps,
		RequestID:        requestID,
	}
}

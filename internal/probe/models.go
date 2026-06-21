package probe

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"llm-api-uptime/internal/model"
)

func FetchModelList(ctx context.Context, provider model.Provider) ([]string, error) {
	switch provider.APIType {
	case model.APITypeOpenAI:
		client := openai.NewClient(
			option.WithBaseURL(strings.TrimRight(provider.BaseURL, "/") + "/v1"),
			option.WithAPIKey(provider.APIKey),
		)

		result, err := client.Models.List(ctx)
		if err != nil {
			return nil, err
		}

		models := make([]string, 0, len(result.Data))
		for _, m := range result.Data {
			if m.ID != "" {
				models = append(models, m.ID)
			}
		}
		return models, nil

	case model.APITypeAnthropic:
		return nil, fmt.Errorf("anthropic does not support model listing via API")

	default:
		return nil, fmt.Errorf("unsupported api type: %s", provider.APIType)
	}
}

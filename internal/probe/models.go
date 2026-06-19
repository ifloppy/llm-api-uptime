package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-api-uptime/internal/model"
)

type modelListResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func FetchModelList(ctx context.Context, provider model.Provider) ([]string, error) {
	var url string
	var req *http.Request
	var err error

	switch provider.APIType {
	case model.APITypeOpenAI:
		url = strings.TrimRight(provider.BaseURL, "/") + "/v1/models"
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	case model.APITypeAnthropic:
		return nil, fmt.Errorf("anthropic does not support model listing via API")

	default:
		return nil, fmt.Errorf("unsupported api type: %s", provider.APIType)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var modelResp modelListResponse
	if err := json.Unmarshal(body, &modelResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	models := make([]string, 0, len(modelResp.Data))
	for _, m := range modelResp.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}

	return models, nil
}

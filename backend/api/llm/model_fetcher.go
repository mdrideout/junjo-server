package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"junjo-server/api/llm/provider"
)

// FetchGeminiModels fetches the latest model list from the Gemini API
func FetchGeminiModels(ctx context.Context) ([]provider.ModelInfo, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	// Call Gemini models API
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Gemini models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var geminiResp struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			Description                string   `json:"description"`
			InputTokenLimit            int      `json:"inputTokenLimit"`
			OutputTokenLimit           int      `json:"outputTokenLimit"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini models response: %w", err)
	}

	// Convert to ModelInfo and filter
	models := make([]provider.ModelInfo, 0, len(geminiResp.Models))
	for _, m := range geminiResp.Models {
		// Filter to only models that support generateContent
		supportsGenerate := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsGenerate = true
				break
			}
		}
		if !supportsGenerate {
			continue
		}

		// Extract model ID from name (e.g., "models/gemini-2.5-flash" -> "gemini-2.5-flash")
		modelID := m.Name
		if len(m.Name) > 7 && m.Name[:7] == "models/" {
			modelID = m.Name[7:]
		}

		models = append(models, provider.ModelInfo{
			ID:              modelID,
			Name:            m.DisplayName,
			Provider:        "gemini",
			Description:     m.Description,
			ContextWindow:   m.InputTokenLimit,
			MaxOutputTokens: m.OutputTokenLimit,
		})
	}

	slog.Debug("Fetched Gemini models from API", "count", len(models))
	return models, nil
}

// FetchOpenAIModels fetches the latest model list from the OpenAI API
func FetchOpenAIModels(ctx context.Context) ([]provider.ModelInfo, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	// Call OpenAI models API
	url := "https://api.openai.com/v1/models"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenAI models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var openaiResp struct {
		Data []struct {
			ID      string `json:"id"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI models response: %w", err)
	}

	// Convert to ModelInfo
	// Filter to chat models (gpt-*) and exclude deprecated/fine-tuned models
	models := make([]provider.ModelInfo, 0, len(openaiResp.Data))
	for _, m := range openaiResp.Data {
		// Only include gpt-* models
		if len(m.ID) < 4 || m.ID[:4] != "gpt-" {
			continue
		}

		// Skip fine-tuned models (contain :ft- in the ID)
		if len(m.ID) > 4 && contains(m.ID, ":ft-") {
			continue
		}

		// Convert Unix timestamp to ISO string
		createdAt := ""
		if m.Created > 0 {
			createdAt = time.Unix(m.Created, 0).Format(time.RFC3339)
		}

		models = append(models, provider.ModelInfo{
			ID:        m.ID,
			Name:      m.ID, // OpenAI doesn't provide display names
			Provider:  "openai",
			CreatedAt: createdAt,
			Metadata: map[string]string{
				"owned_by": m.OwnedBy,
			},
		})
	}

	slog.Debug("Fetched OpenAI models from API", "count", len(models))
	return models, nil
}

// FetchAnthropicModels fetches the latest model list from the Anthropic API
func FetchAnthropicModels(ctx context.Context) ([]provider.ModelInfo, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	// Call Anthropic models API
	url := "https://api.anthropic.com/v1/models"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Anthropic models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var anthropicResp struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			CreatedAt   string `json:"created_at"`
			Type        string `json:"type"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic models response: %w", err)
	}

	// Convert to ModelInfo
	// Filter to only message models (exclude embedding, etc.)
	models := make([]provider.ModelInfo, 0, len(anthropicResp.Data))
	for _, m := range anthropicResp.Data {
		// Only include models of type "model" (chat models)
		if m.Type != "" && m.Type != "model" {
			continue
		}

		name := m.DisplayName
		if name == "" {
			name = m.ID
		}

		models = append(models, provider.ModelInfo{
			ID:        m.ID,
			Name:      name,
			Provider:  "anthropic",
			CreatedAt: m.CreatedAt,
		})
	}

	slog.Debug("Fetched Anthropic models from API", "count", len(models))
	return models, nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != "" && substr != "" &&
		(s == substr || len(s) > len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

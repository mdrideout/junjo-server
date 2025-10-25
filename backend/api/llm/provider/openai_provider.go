package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/services/openai"
)

// OpenAIProvider implements the LLMProvider interface for OpenAI
type OpenAIProvider struct {
	apiKey string
	client *openai.OpenAI
}

// NewOpenAIProvider creates a new OpenAI provider instance
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	client := openai.New(apiKey)
	return &OpenAIProvider{
		apiKey: apiKey,
		client: client,
	}
}

// GenerateContent generates content using OpenAI
func (p *OpenAIProvider) GenerateContent(ctx context.Context, req *UnifiedRequest) (*UnifiedResponse, error) {
	if err := p.ValidateAPIKey(); err != nil {
		return nil, err
	}

	// Build the generator request
	generator := p.client.Generator().WithContext(ctx).Model(gen.Model{Name: req.Model})

	// Build system prompt
	systemPrompt := req.SystemPrompt
	if req.JSONMode {
		jsonInstr := "Respond with valid JSON only."
		if systemPrompt != "" {
			systemPrompt += " " + jsonInstr
		} else {
			systemPrompt = jsonInstr
		}
	}
	if systemPrompt != "" {
		generator = generator.System(systemPrompt)
	}

	// Apply parameters
	if req.Temperature > 0 {
		generator = generator.Temperature(req.Temperature)
	}
	if req.MaxTokens > 0 {
		generator = generator.MaxTokens(req.MaxTokens)
	}
	if req.TopP > 0 {
		generator = generator.TopP(req.TopP)
	}
	if len(req.StopSequences) > 0 {
		for _, stop := range req.StopSequences {
			generator = generator.StopAt(stop)
		}
	}

	// Execute the request (Prompt both adds the prompt and executes)
	response, err := generator.Prompt(prompt.AsUser(req.Prompt))
	if err != nil {
		return nil, fmt.Errorf("openai generation failed: %w", err)
	}

	// Extract text from response
	text, err := response.AsText()
	if err != nil {
		return nil, fmt.Errorf("failed to extract text from response: %w", err)
	}

	return &UnifiedResponse{
		Text:     text,
		Provider: string(ProviderOpenAI),
		Model:    req.Model,
	}, nil
}

// SupportedModels returns the list of supported OpenAI models
func (p *OpenAIProvider) SupportedModels() []ModelInfo {
	return OpenAIModels
}

// FetchAvailableModels fetches the latest model list from the OpenAI API
func (p *OpenAIProvider) FetchAvailableModels(ctx context.Context) ([]ModelInfo, error) {
	if err := p.ValidateAPIKey(); err != nil {
		return nil, err
	}

	// Call OpenAI models API
	url := "https://api.openai.com/v1/models"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

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

	// Convert to ModelInfo and filter to only gpt-* models
	models := make([]ModelInfo, 0)
	for _, m := range openaiResp.Data {
		// Filter to only chat models (gpt-*)
		if !strings.HasPrefix(m.ID, "gpt-") {
			continue
		}

		createdAt := ""
		if m.Created > 0 {
			createdAt = time.Unix(m.Created, 0).Format(time.RFC3339)
		}

		models = append(models, ModelInfo{
			ID:        m.ID,
			Name:      m.ID, // OpenAI doesn't provide display names
			Provider:  string(ProviderOpenAI),
			CreatedAt: createdAt,
		})
	}

	return models, nil
}

// ValidateAPIKey checks if the API key is available
func (p *OpenAIProvider) ValidateAPIKey() error {
	if p.apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}
	return nil
}

// GetBellmanClient returns the underlying Bellman client
func (p *OpenAIProvider) GetBellmanClient() interface{} {
	return p.client
}

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/services/anthropic"
)

// AnthropicProvider implements the LLMProvider interface for Anthropic Claude
type AnthropicProvider struct {
	apiKey string
	client *anthropic.Anthropic
}

// NewAnthropicProvider creates a new Anthropic provider instance
func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	client := anthropic.New(apiKey)
	return &AnthropicProvider{
		apiKey: apiKey,
		client: client,
	}
}

// GenerateContent generates content using Anthropic Claude
func (p *AnthropicProvider) GenerateContent(ctx context.Context, req *UnifiedRequest) (*UnifiedResponse, error) {
	if err := p.ValidateAPIKey(); err != nil {
		return nil, err
	}

	// Build the generator request
	generator := p.client.Generator().WithContext(ctx).Model(gen.Model{Name: req.Model})

	// Build system prompt (Anthropic requires JSON instructions in system prompt)
	systemPrompt := req.SystemPrompt
	if req.JSONMode {
		jsonInstr := "Please respond with valid JSON only, no other text."
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
		return nil, fmt.Errorf("anthropic generation failed: %w", err)
	}

	// Extract text from response
	text, err := response.AsText()
	if err != nil {
		return nil, fmt.Errorf("failed to extract text from response: %w", err)
	}

	return &UnifiedResponse{
		Text:     text,
		Provider: string(ProviderAnthropic),
		Model:    req.Model,
	}, nil
}

// SupportedModels returns the list of supported Anthropic models
func (p *AnthropicProvider) SupportedModels() []ModelInfo {
	return AnthropicModels
}

// FetchAvailableModels fetches the latest model list from the Anthropic API
func (p *AnthropicProvider) FetchAvailableModels(ctx context.Context) ([]ModelInfo, error) {
	if err := p.ValidateAPIKey(); err != nil {
		return nil, err
	}

	// Call Anthropic models API
	url := "https://api.anthropic.com/v1/models"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("x-api-key", p.apiKey)
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
		HasMore bool `json:"has_more"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic models response: %w", err)
	}

	// Convert to ModelInfo
	models := make([]ModelInfo, 0, len(anthropicResp.Data))
	for _, m := range anthropicResp.Data {
		models = append(models, ModelInfo{
			ID:        m.ID,
			Name:      m.DisplayName,
			Provider:  string(ProviderAnthropic),
			CreatedAt: m.CreatedAt,
		})
	}

	return models, nil
}

// ValidateAPIKey checks if the API key is available
func (p *AnthropicProvider) ValidateAPIKey() error {
	if p.apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}
	return nil
}

// GetBellmanClient returns the underlying Bellman client
func (p *AnthropicProvider) GetBellmanClient() interface{} {
	return p.client
}

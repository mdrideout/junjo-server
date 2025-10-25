package provider

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// ProviderType represents the LLM provider
type ProviderType string

const (
	ProviderGemini    ProviderType = "gemini"
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
)

// ModelInfo contains metadata about a model
type ModelInfo struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Provider        string            `json:"provider"`
	Description     string            `json:"description,omitempty"`
	ContextWindow   int               `json:"contextWindow,omitempty"`
	MaxOutputTokens int               `json:"maxOutputTokens,omitempty"`
	CreatedAt       string            `json:"createdAt,omitempty"`
	Capabilities    []string          `json:"capabilities,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// UnifiedRequest represents a provider-agnostic LLM request
type UnifiedRequest struct {
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	Prompt           string  `json:"prompt"`
	SystemPrompt     string  `json:"systemPrompt,omitempty"`
	JSONMode         bool    `json:"jsonMode,omitempty"`
	Temperature      float64 `json:"temperature,omitempty"`
	MaxTokens        int     `json:"maxTokens,omitempty"`
	TopP             float64 `json:"topP,omitempty"`
	TopK             int     `json:"topK,omitempty"`
	StopSequences    []string `json:"stopSequences,omitempty"`
}

// UnifiedResponse represents a provider-agnostic LLM response
type UnifiedResponse struct {
	Text     string `json:"text"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// LLMProvider defines the interface that all provider implementations must satisfy
type LLMProvider interface {
	// GenerateContent generates content using the provider's LLM
	GenerateContent(ctx context.Context, req *UnifiedRequest) (*UnifiedResponse, error)

	// SupportedModels returns the list of models supported by this provider
	SupportedModels() []ModelInfo

	// FetchAvailableModels fetches the latest model list from the provider API
	FetchAvailableModels(ctx context.Context) ([]ModelInfo, error)

	// ValidateAPIKey checks if the API key is available
	ValidateAPIKey() error

	// GetBellmanClient returns the underlying client (can be nil if not using Bellman)
	GetBellmanClient() interface{}
}

// ProviderFactory creates provider instances
type ProviderFactory struct {
	providers map[ProviderType]LLMProvider
}

// NewProviderFactory creates a new provider factory with all available providers
func NewProviderFactory() *ProviderFactory {
	factory := &ProviderFactory{
		providers: make(map[ProviderType]LLMProvider),
	}

	// Initialize Gemini provider if API key is available
	if geminiKey := os.Getenv("GEMINI_API_KEY"); geminiKey != "" {
		factory.providers[ProviderGemini] = NewGeminiProvider(geminiKey)
	}

	// Initialize OpenAI provider if API key is available
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
		factory.providers[ProviderOpenAI] = NewOpenAIProvider(openaiKey)
	}

	// Initialize Anthropic provider if API key is available
	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		factory.providers[ProviderAnthropic] = NewAnthropicProvider(anthropicKey)
	}

	return factory
}

// GetProvider returns a provider by type
func (f *ProviderFactory) GetProvider(providerType ProviderType) (LLMProvider, error) {
	provider, exists := f.providers[providerType]
	if !exists {
		return nil, fmt.Errorf("provider %s not available or API key not configured", providerType)
	}
	return provider, nil
}

// GetAvailableProviders returns a list of all available providers
func (f *ProviderFactory) GetAvailableProviders() []ProviderType {
	providers := make([]ProviderType, 0, len(f.providers))
	for providerType := range f.providers {
		providers = append(providers, providerType)
	}
	return providers
}

// GetAllModels returns all models from all available providers
func (f *ProviderFactory) GetAllModels() []ModelInfo {
	var models []ModelInfo
	for _, provider := range f.providers {
		models = append(models, provider.SupportedModels()...)
	}
	return models
}

// GetModelsByProvider returns models for a specific provider
// First checks cache, then fetches from API if needed, falls back to hardcoded list
func (f *ProviderFactory) GetModelsByProvider(providerType ProviderType) ([]ModelInfo, error) {
	provider, err := f.GetProvider(providerType)
	if err != nil {
		return nil, err
	}

	// Try cache first
	if cached, ok := globalModelCache.Get(providerType); ok {
		slog.Debug("Using cached models", "provider", providerType)
		return cached, nil
	}

	// Try fetching from provider API
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if models, err := provider.FetchAvailableModels(ctx); err == nil {
		slog.Debug("Fetched models from provider API", "provider", providerType, "count", len(models))
		globalModelCache.Set(providerType, models)
		return models, nil
	} else {
		// Log the error but continue with fallback
		slog.Warn("Failed to fetch models from provider API, using fallback", "provider", providerType, "error", err)
	}

	// Fallback to hardcoded list
	slog.Debug("Using hardcoded fallback models", "provider", providerType)
	return provider.SupportedModels(), nil
}

// RefreshModels forces a refresh of models from the provider API
func (f *ProviderFactory) RefreshModels(providerType ProviderType) ([]ModelInfo, error) {
	provider, err := f.GetProvider(providerType)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := provider.FetchAvailableModels(ctx)
	if err != nil {
		return nil, err
	}

	globalModelCache.Set(providerType, models)
	slog.Debug("Refreshed models from provider API", "provider", providerType, "count", len(models))
	return models, nil
}

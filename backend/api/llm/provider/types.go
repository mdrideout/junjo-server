package provider

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

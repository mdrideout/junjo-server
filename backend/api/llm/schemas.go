package llm

// GeminiPart represents a part of a content message.
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiContent represents the content of a message.
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GenerationConfig specifies the generation parameters for the model.
// https://ai.google.dev/api/generate-content#v1beta.GenerationConfig
type GenerationConfig struct {
	StopSequences    []string `json:"stopSequences,omitempty"`
	ResponseMimeType string   `json:"responseMimeType,omitempty"`
	Temperature      float64  `json:"temperature,omitempty"`
	TopP             float64  `json:"topP,omitempty"`
	TopK             int      `json:"topK,omitempty"`
}

// SystemInstruction provides system-level instructions to the model.
type SystemInstruction struct {
	Parts []GeminiPart `json:"parts"`
}

// GeminiRequest is the request body for the Gemini API.
// Deprecated: Use UnifiedLLMRequest for new implementations
type GeminiRequest struct {
	Model             string             `json:"model,omitempty"`
	Contents          []GeminiContent    `json:"contents"`
	GenerationConfig  *GenerationConfig  `json:"generationConfig,omitempty"`
	SystemInstruction *SystemInstruction `json:"system_instruction,omitempty"`
}

// UnifiedLLMRequest represents a provider-agnostic LLM request
type UnifiedLLMRequest struct {
	Provider      string   `json:"provider"`
	Model         string   `json:"model"`
	Prompt        string   `json:"prompt"`
	SystemPrompt  string   `json:"systemPrompt,omitempty"`
	JSONMode      bool     `json:"jsonMode,omitempty"`
	Temperature   float64  `json:"temperature,omitempty"`
	MaxTokens     int      `json:"maxTokens,omitempty"`
	TopP          float64  `json:"topP,omitempty"`
	TopK          int      `json:"topK,omitempty"`
	StopSequences []string `json:"stopSequences,omitempty"`
}

// UnifiedLLMResponse represents a provider-agnostic LLM response
type UnifiedLLMResponse struct {
	Text     string `json:"text"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// ProviderInfo contains information about an available provider
type ProviderInfo struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

// ModelInfo contains information about a model
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

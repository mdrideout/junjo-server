package openai

// OpenAIMessage represents a message in the OpenAI chat format
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIJSONSchema represents a JSON schema for structured output
type OpenAIJSONSchema struct {
	Name   string                 `json:"name"`
	Strict *bool                  `json:"strict,omitempty"`
	Schema map[string]interface{} `json:"schema"`
}

// OpenAIResponseFormat specifies the response format for OpenAI
type OpenAIResponseFormat struct {
	Type       string            `json:"type"` // "json_object", "text", or "json_schema"
	JSONSchema *OpenAIJSONSchema `json:"json_schema,omitempty"`
}

// OpenAIRequest represents a request to OpenAI's chat completions API
type OpenAIRequest struct {
	Model               string                `json:"model"`
	Messages            []OpenAIMessage       `json:"messages"`
	Temperature         *float64              `json:"temperature,omitempty"`
	MaxTokens           *int                  `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                  `json:"max_completion_tokens,omitempty"`
	ReasoningEffort     *string               `json:"reasoning_effort,omitempty"` // "minimal", "low", "medium", "high"
	TopP                *float64              `json:"top_p,omitempty"`
	StopSequences       []string              `json:"stop,omitempty"`
	ResponseFormat      *OpenAIResponseFormat `json:"response_format,omitempty"`
}

// OpenAIChoice represents a choice in the OpenAI response
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAIUsage represents token usage in the OpenAI response
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIResponse represents a response from OpenAI's chat completions API
type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

package anthropic

// AnthropicMessage represents a message in the Anthropic Messages API
type AnthropicMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"` // Text content of the message
}

// AnthropicToolChoice represents the tool choice configuration
type AnthropicToolChoice struct {
	Type string `json:"type"` // "auto", "any", or "tool"
	Name string `json:"name,omitempty"`
}

// AnthropicTool represents a tool that can be used for structured output
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// AnthropicThinking represents the thinking configuration
type AnthropicThinking struct {
	Type         string `json:"type"` // "enabled"
	BudgetTokens int    `json:"budget_tokens"` // >= 1024 and < max_tokens
}

// AnthropicRequest represents a request to Anthropic's Messages API
type AnthropicRequest struct {
	Model              string                `json:"model"`
	Messages           []AnthropicMessage    `json:"messages"`
	System             string                `json:"system,omitempty"`
	MaxTokens          int                   `json:"max_tokens"`
	Temperature        *float64              `json:"temperature,omitempty"`
	TopP               *float64              `json:"top_p,omitempty"`
	TopK               *int                  `json:"top_k,omitempty"`
	StopSequences      []string              `json:"stop_sequences,omitempty"`
	Thinking           *AnthropicThinking    `json:"thinking,omitempty"`
	Tools              []AnthropicTool       `json:"tools,omitempty"`
	ToolChoice         *AnthropicToolChoice  `json:"tool_choice,omitempty"`

	// Extension for JSON mode (not part of Anthropic API)
	// When true, will automatically configure tool calling for structured output
	JSONMode bool `json:"jsonMode,omitempty"`
}

// AnthropicContentBlock represents a content block in the response
type AnthropicContentBlock struct {
	Type  string                 `json:"type"` // "text" or "tool_use"
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

// AnthropicUsage represents token usage in the Anthropic response
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicResponse represents a response from Anthropic's Messages API
type AnthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence,omitempty"`
	Usage        AnthropicUsage          `json:"usage"`
}

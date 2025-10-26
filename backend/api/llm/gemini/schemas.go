package gemini

// GeminiPart represents a part of content in a message
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiContent represents content in the Gemini API
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiThinkingConfig represents the thinking configuration
type GeminiThinkingConfig struct {
	ThinkingBudget  *int  `json:"thinkingBudget,omitempty"`  // -1 = dynamic, 0 = disabled, 1-32768 = specific budget
	IncludeThoughts *bool `json:"includeThoughts,omitempty"` // Include thought summaries in response
}

// GeminiGenerationConfig represents the generation configuration
type GeminiGenerationConfig struct {
	Temperature         *float64               `json:"temperature,omitempty"`
	TopP                *float64               `json:"topP,omitempty"`
	TopK                *int                   `json:"topK,omitempty"`
	MaxOutputTokens     *int                   `json:"maxOutputTokens,omitempty"`
	StopSequences       []string               `json:"stopSequences,omitempty"`
	ResponseMimeType    string                 `json:"responseMimeType,omitempty"`
	ResponseJSONSchema  map[string]interface{} `json:"response_json_schema,omitempty"`
	ThinkingConfig      *GeminiThinkingConfig  `json:"thinkingConfig,omitempty"`
}

// GeminiSystemInstruction represents system-level instructions
type GeminiSystemInstruction struct {
	Parts []GeminiPart `json:"parts"`
}

// GeminiRequest represents a request to Gemini's GenerateContent API
type GeminiRequest struct {
	Model             string                   `json:"model,omitempty"`
	Contents          []GeminiContent          `json:"contents"`
	GenerationConfig  *GeminiGenerationConfig  `json:"generationConfig,omitempty"`
	SystemInstruction *GeminiSystemInstruction `json:"system_instruction,omitempty"`
}

// GeminiCandidate represents a response candidate
type GeminiCandidate struct {
	Content      GeminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
	Index        int           `json:"index"`
}

// GeminiUsageMetadata represents token usage
type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// GeminiResponse represents a response from Gemini's GenerateContent API
type GeminiResponse struct {
	Candidates    []GeminiCandidate    `json:"candidates"`
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

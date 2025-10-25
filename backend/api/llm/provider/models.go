package provider

// GeminiModels contains all supported Gemini models
var GeminiModels = []ModelInfo{
	{
		ID:          "gemini-2.5-pro",
		Name:        "Gemini 2.5 Pro",
		Provider:    "gemini",
		Description: "Most capable Gemini model",
	},
	{
		ID:          "gemini-2.5-flash",
		Name:        "Gemini 2.5 Flash",
		Provider:    "gemini",
		Description: "Fast and efficient Gemini model",
	},
	{
		ID:          "gemini-2.5-flash-lite",
		Name:        "Gemini 2.5 Flash Lite",
		Provider:    "gemini",
		Description: "Lightweight Gemini model",
	},
	{
		ID:          "gemini-2.0-flash",
		Name:        "Gemini 2.0 Flash",
		Provider:    "gemini",
		Description: "Previous generation fast model",
	},
	{
		ID:          "gemini-2.0-flash-lite",
		Name:        "Gemini 2.0 Flash Lite",
		Provider:    "gemini",
		Description: "Previous generation lite model",
	},
}

// OpenAIModels contains all supported OpenAI models
var OpenAIModels = []ModelInfo{
	{
		ID:          "gpt-4o",
		Name:        "GPT-4o",
		Provider:    "openai",
		Description: "Most capable GPT-4 model with vision",
	},
	{
		ID:          "gpt-4o-mini",
		Name:        "GPT-4o Mini",
		Provider:    "openai",
		Description: "Faster and more affordable GPT-4 model",
	},
	{
		ID:          "gpt-4-turbo",
		Name:        "GPT-4 Turbo",
		Provider:    "openai",
		Description: "Previous GPT-4 turbo model",
	},
	{
		ID:          "gpt-4",
		Name:        "GPT-4",
		Provider:    "openai",
		Description: "Original GPT-4 model",
	},
	{
		ID:          "gpt-3.5-turbo",
		Name:        "GPT-3.5 Turbo",
		Provider:    "openai",
		Description: "Fast and efficient GPT-3.5 model",
	},
}

// AnthropicModels contains all supported Anthropic models
var AnthropicModels = []ModelInfo{
	{
		ID:          "claude-3-5-sonnet-20241022",
		Name:        "Claude 3.5 Sonnet",
		Provider:    "anthropic",
		Description: "Most capable Claude model",
	},
	{
		ID:          "claude-3-5-haiku-20241022",
		Name:        "Claude 3.5 Haiku",
		Provider:    "anthropic",
		Description: "Fastest Claude model",
	},
	{
		ID:          "claude-3-opus-20240229",
		Name:        "Claude 3 Opus",
		Provider:    "anthropic",
		Description: "Previous generation powerful model",
	},
	{
		ID:          "claude-3-sonnet-20240229",
		Name:        "Claude 3 Sonnet",
		Provider:    "anthropic",
		Description: "Balanced Claude 3 model",
	},
	{
		ID:          "claude-3-haiku-20240307",
		Name:        "Claude 3 Haiku",
		Provider:    "anthropic",
		Description: "Previous generation fast model",
	},
}

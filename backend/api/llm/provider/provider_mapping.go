package provider

import "strings"

// MapOtelProviderToInternal maps OpenInference provider names to internal provider names
// OpenInference uses "google" for Gemini, but we use "gemini" internally
func MapOtelProviderToInternal(otelProvider string) string {
	mappings := map[string]string{
		"google":     "gemini",
		"openai":     "openai",
		"anthropic":  "anthropic",
		"gemini":     "gemini", // Allow "gemini" directly for compatibility
		"cohere":     "cohere",
		"mistralai":  "mistralai",
		"azure":      "azure",
		"aws":        "aws",
	}

	normalized := strings.ToLower(strings.TrimSpace(otelProvider))
	if mapped, ok := mappings[normalized]; ok {
		return mapped
	}

	// Return the normalized provider name if no mapping exists
	return normalized
}

// MapInternalProviderToOtel maps internal provider names back to OpenInference standard names
func MapInternalProviderToOtel(internalProvider string) string {
	mappings := map[string]string{
		"gemini":     "google",
		"openai":     "openai",
		"anthropic":  "anthropic",
		"cohere":     "cohere",
		"mistralai":  "mistralai",
		"azure":      "azure",
		"aws":        "aws",
	}

	normalized := strings.ToLower(strings.TrimSpace(internalProvider))
	if mapped, ok := mappings[normalized]; ok {
		return mapped
	}

	// Return the normalized provider name if no mapping exists
	return normalized
}

// GetProviderDisplayName returns a human-friendly display name for a provider
func GetProviderDisplayName(provider string) string {
	displayNames := map[string]string{
		"gemini":     "Google Gemini",
		"openai":     "OpenAI",
		"anthropic":  "Anthropic",
		"cohere":     "Cohere",
		"mistralai":  "Mistral AI",
		"azure":      "Azure",
		"aws":        "AWS Bedrock",
		"google":     "Google Gemini", // Handle both forms
	}

	normalized := strings.ToLower(strings.TrimSpace(provider))
	if displayName, ok := displayNames[normalized]; ok {
		return displayName
	}

	// Return capitalized version as fallback
	if len(normalized) > 0 {
		return strings.ToUpper(normalized[:1]) + normalized[1:]
	}

	return provider
}

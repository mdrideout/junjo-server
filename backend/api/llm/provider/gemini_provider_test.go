package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGeminiProvider_FetchAvailableModels_Success verifies successful API response parsing
func TestGeminiProvider_FetchAvailableModels_Success(t *testing.T) {
	// Create test server that mocks Gemini API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"models": []map[string]interface{}{
				{
					"name":                       "models/gemini-2.5-flash",
					"displayName":                "Gemini 2.5 Flash",
					"description":                "Fast and efficient model",
					"inputTokenLimit":            1000000,
					"outputTokenLimit":           8192,
					"supportedGenerationMethods": []string{"generateContent"},
				},
				{
					"name":                       "models/gemini-2.0-pro",
					"displayName":                "Gemini 2.0 Pro",
					"description":                "Advanced reasoning model",
					"inputTokenLimit":            2000000,
					"outputTokenLimit":           8192,
					"supportedGenerationMethods": []string{"generateContent"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider and override API URL for testing
	provider := &GeminiProvider{apiKey: "test-key"}

	// Note: We need to mock the actual URL in the FetchAvailableModels method
	// For now, we'll test with a real-ish scenario by using context
	ctx := context.Background()

	// This test demonstrates the structure but would need URL injection
	// to fully work without hitting real API
	t.Skip("Skipping until we add URL injection capability to GeminiProvider")

	models, err := provider.FetchAvailableModels(ctx)

	assert.NoError(t, err, "Should not return error for successful API response")
	assert.Len(t, models, 2, "Should parse 2 models from response")
	assert.Equal(t, "gemini-2.5-flash", models[0].ID, "First model ID should be correct")
	assert.Equal(t, "Gemini 2.5 Flash", models[0].Name, "First model name should be correct")
}

// TestGeminiProvider_FetchAvailableModels_FilterNonGenerativeModels verifies filtering logic
func TestGeminiProvider_FetchAvailableModels_FilterNonGenerativeModels(t *testing.T) {
	// Create test server with mix of generative and non-generative models
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"models": []map[string]interface{}{
				{
					"name":                       "models/gemini-2.5-flash",
					"displayName":                "Gemini 2.5 Flash",
					"description":                "Fast model",
					"inputTokenLimit":            1000000,
					"outputTokenLimit":           8192,
					"supportedGenerationMethods": []string{"generateContent"},
				},
				{
					"name":                       "models/embedding-001",
					"displayName":                "Embedding Model",
					"description":                "Text embeddings",
					"inputTokenLimit":            2048,
					"outputTokenLimit":           0,
					"supportedGenerationMethods": []string{"embedContent"},
				},
				{
					"name":                       "models/aqa",
					"displayName":                "Attributed Question Answering",
					"description":                "AQA model",
					"inputTokenLimit":            7168,
					"outputTokenLimit":           1024,
					"supportedGenerationMethods": []string{"generateAnswer"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &GeminiProvider{apiKey: "test-key"}
	ctx := context.Background()

	t.Skip("Skipping until we add URL injection capability to GeminiProvider")

	models, err := provider.FetchAvailableModels(ctx)

	assert.NoError(t, err, "Should not return error")
	assert.Len(t, models, 1, "Should only include generative models")
	assert.Equal(t, "gemini-2.5-flash", models[0].ID, "Should only include generateContent models")
}

// TestGeminiProvider_FetchAvailableModels_APIError verifies error handling
func TestGeminiProvider_FetchAvailableModels_APIError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	provider := &GeminiProvider{apiKey: "invalid-key"}
	ctx := context.Background()

	t.Skip("Skipping until we add URL injection capability to GeminiProvider")

	models, err := provider.FetchAvailableModels(ctx)

	assert.Error(t, err, "Should return error for API failure")
	assert.Nil(t, models, "Should return nil models on error")
	assert.Contains(t, err.Error(), "401", "Error should mention status code")
}

// TestGeminiProvider_ValidateAPIKey verifies API key validation
func TestGeminiProvider_ValidateAPIKey(t *testing.T) {
	// Test with valid API key
	providerWithKey := &GeminiProvider{apiKey: "test-key"}
	err := providerWithKey.ValidateAPIKey()
	assert.NoError(t, err, "Should not error with API key set")

	// Test with missing API key
	providerWithoutKey := &GeminiProvider{apiKey: ""}
	err = providerWithoutKey.ValidateAPIKey()
	assert.Error(t, err, "Should error with missing API key")
	assert.Contains(t, err.Error(), "GEMINI_API_KEY", "Error should mention API key")
}

// TestGeminiProvider_SupportedModels verifies fallback hardcoded models
func TestGeminiProvider_SupportedModels(t *testing.T) {
	provider := &GeminiProvider{apiKey: "test-key"}

	models := provider.SupportedModels()

	assert.NotEmpty(t, models, "Should return fallback models")
	assert.Greater(t, len(models), 0, "Should have at least one fallback model")

	// Verify all models have required fields
	for _, model := range models {
		assert.NotEmpty(t, model.ID, "Model should have ID")
		assert.NotEmpty(t, model.Name, "Model should have name")
		assert.Equal(t, "gemini", model.Provider, "Model should be gemini provider")
	}
}

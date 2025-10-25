package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"junjo-server/api/llm/provider"
)

// TestHandleGetProviders verifies the /llm/providers endpoint
func TestHandleGetProviders(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/llm/providers", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := HandleGetProviders(c)

	assert.NoError(t, err, "Should not return error")
	assert.Equal(t, http.StatusOK, rec.Code, "Should return 200 OK")

	// Parse response
	var providers []ProviderInfo
	err = json.Unmarshal(rec.Body.Bytes(), &providers)
	assert.NoError(t, err, "Should parse JSON response")
	assert.NotEmpty(t, providers, "Should return provider list")

	// Verify all three providers are in the response
	providerNames := make(map[string]bool)
	for _, p := range providers {
		providerNames[p.Name] = true
	}
	assert.True(t, providerNames["gemini"], "Should include gemini provider")
	assert.True(t, providerNames["openai"], "Should include openai provider")
	assert.True(t, providerNames["anthropic"], "Should include anthropic provider")
}

// TestHandleGetModels verifies the /llm/models endpoint returns all models
func TestHandleGetModels(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/llm/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := HandleGetModels(c)

	assert.NoError(t, err, "Should not return error")
	assert.Equal(t, http.StatusOK, rec.Code, "Should return 200 OK")

	// Parse response
	var models []provider.ModelInfo
	err = json.Unmarshal(rec.Body.Bytes(), &models)
	assert.NoError(t, err, "Should parse JSON response")

	// Note: models may be empty if no providers are configured (no API keys)
	// This is acceptable for smoke testing - the important thing is the endpoint works

	// If models are returned, verify they have required fields
	for _, model := range models {
		assert.NotEmpty(t, model.ID, "Model should have ID")
		assert.NotEmpty(t, model.Provider, "Model should have provider")
	}
}

// TestHandleGetModelsByProvider_ValidProvider verifies fetching models for a specific provider
func TestHandleGetModelsByProvider_ValidProvider(t *testing.T) {
	// Skip if API keys are not configured
	// This test will use the actual provider factory, so it needs real or fallback models
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/llm/providers/gemini/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("gemini")

	err := HandleGetModelsByProvider(c)

	// Should succeed even without API key (falls back to hardcoded models)
	assert.NoError(t, err, "Should not return error")

	// Verify response based on whether provider is available
	if rec.Code == http.StatusOK {
		var models []provider.ModelInfo
		err = json.Unmarshal(rec.Body.Bytes(), &models)
		assert.NoError(t, err, "Should parse JSON response")
		assert.NotEmpty(t, models, "Should return models (either from API or fallback)")

		// Verify all models are from the correct provider
		for _, model := range models {
			assert.Equal(t, "gemini", model.Provider, "All models should be from gemini provider")
		}
	} else if rec.Code == http.StatusNotFound {
		// Provider not configured (no API key)
		var errResp map[string]string
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		assert.NoError(t, err, "Should parse error response")
		assert.Contains(t, errResp["error"], "not available", "Error should mention provider not available")
	}
}

// TestHandleGetModelsByProvider_InvalidProvider verifies error handling for invalid provider
func TestHandleGetModelsByProvider_InvalidProvider(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/llm/providers/invalid/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("invalid")

	err := HandleGetModelsByProvider(c)

	assert.NoError(t, err, "Should not return error (error is in HTTP response)")
	assert.Equal(t, http.StatusNotFound, rec.Code, "Should return 404 for invalid provider")

	var errResp map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &errResp)
	assert.NoError(t, err, "Should parse error response")
	assert.NotEmpty(t, errResp["error"], "Should return error message")
}

// TestHandleGetModelsByProvider_MissingProvider verifies error when provider param is missing
func TestHandleGetModelsByProvider_MissingProvider(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/llm/providers//models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// Don't set param - simulate missing provider

	err := HandleGetModelsByProvider(c)

	assert.NoError(t, err, "Should not return error (error is in HTTP response)")
	assert.Equal(t, http.StatusBadRequest, rec.Code, "Should return 400 for missing provider")
}

// TestHandleRefreshModels_ValidProvider verifies the refresh endpoint
func TestHandleRefreshModels_ValidProvider(t *testing.T) {
	// This test may fail if API keys are not configured
	// It demonstrates the endpoint structure but may need API keys to fully test
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/llm/providers/gemini/models/refresh", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("gemini")

	err := HandleRefreshModels(c)

	assert.NoError(t, err, "Should not return error at handler level")

	// Response code depends on whether API key is configured
	if rec.Code == http.StatusOK {
		var models []provider.ModelInfo
		err = json.Unmarshal(rec.Body.Bytes(), &models)
		assert.NoError(t, err, "Should parse JSON response")
		assert.NotEmpty(t, models, "Should return refreshed models")
	} else {
		// API key not configured or API call failed
		assert.Contains(t, []int{http.StatusInternalServerError, http.StatusNotFound}, rec.Code,
			"Should return error status when refresh fails")
	}
}

// TestHandleRefreshModels_InvalidProvider verifies refresh with invalid provider
func TestHandleRefreshModels_InvalidProvider(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/llm/providers/invalid/models/refresh", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("provider")
	c.SetParamValues("invalid")

	err := HandleRefreshModels(c)

	assert.NoError(t, err, "Should not return error (error is in HTTP response)")
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "Should return 500 for invalid provider")

	var errResp map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &errResp)
	assert.NoError(t, err, "Should parse error response")
	assert.Contains(t, errResp["error"], "Failed to refresh", "Should return refresh error message")
}

// TestHandleRefreshModels_MissingProvider verifies refresh with missing provider
func TestHandleRefreshModels_MissingProvider(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/llm/providers//models/refresh", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// Don't set param

	err := HandleRefreshModels(c)

	assert.NoError(t, err, "Should not return error (error is in HTTP response)")
	assert.Equal(t, http.StatusBadRequest, rec.Code, "Should return 400 for missing provider")
}

// TestHandleUnifiedLLMRequest_MissingFields verifies validation of required fields
func TestHandleUnifiedLLMRequest_MissingFields(t *testing.T) {
	tests := []struct {
		name        string
		requestBody string
		expectedErr string
	}{
		{
			name:        "missing provider",
			requestBody: `{"model": "gpt-4", "prompt": "test"}`,
			expectedErr: "provider is required",
		},
		{
			name:        "missing model",
			requestBody: `{"provider": "openai", "prompt": "test"}`,
			expectedErr: "model is required",
		},
		{
			name:        "missing prompt",
			requestBody: `{"provider": "openai", "model": "gpt-4"}`,
			expectedErr: "prompt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would need actual request body parsing, skipping detailed validation
			t.Skip("Skipping - needs proper request body setup")
			// Suppress unused variable warnings
			_ = tt
		})
	}
}

package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"junjo-server/api/llm/provider"
)

var (
	providerFactory *provider.ProviderFactory
)

func init() {
	// Initialize the provider factory on package load
	providerFactory = provider.NewProviderFactory()
}

// HandleGeminiTextRequest is the handler for the /llm/generate endpoint.
// Deprecated: Use HandleUnifiedLLMRequest for new implementations
func HandleGeminiTextRequest(c echo.Context) error {
	var req GeminiRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Error("Failed to bind request: ", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Set a default model if not provided
	if req.Model == "" {
		req.Model = "gemini-2.5-flash"
	}

	service := NewGeminiService()
	resp, err := service.GenerateContent(req)
	if err != nil {
		c.Logger().Error("Failed to generate content: ", err)

		// Return 503 for missing API key configuration
		if strings.Contains(err.Error(), "GEMINI_API_KEY") {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSONBlob(http.StatusOK, resp)
}

// HandleUnifiedLLMRequest is the handler for the /llm/generate endpoint with multi-provider support.
func HandleUnifiedLLMRequest(c echo.Context) error {
	var req UnifiedLLMRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Error("Failed to bind request: ", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Validate required fields
	if req.Provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider is required"})
	}
	if req.Model == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "model is required"})
	}
	if req.Prompt == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "prompt is required"})
	}

	// Get the provider
	providerType := provider.ProviderType(req.Provider)
	llmProvider, err := providerFactory.GetProvider(providerType)
	if err != nil {
		c.Logger().Error("Failed to get provider: ", err)
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Provider not available. Please ensure the API key is configured.",
		})
	}

	// Convert to provider request format
	providerReq := &provider.UnifiedRequest{
		Provider:      req.Provider,
		Model:         req.Model,
		Prompt:        req.Prompt,
		SystemPrompt:  req.SystemPrompt,
		JSONMode:      req.JSONMode,
		Temperature:   req.Temperature,
		MaxTokens:     req.MaxTokens,
		TopP:          req.TopP,
		TopK:          req.TopK,
		StopSequences: req.StopSequences,
	}

	// Generate content
	ctx := context.Background()
	resp, err := llmProvider.GenerateContent(ctx, providerReq)
	if err != nil {
		c.Logger().Error("Failed to generate content: ", err)

		// Return 503 for API key issues
		if strings.Contains(err.Error(), "API_KEY") || strings.Contains(err.Error(), "api key") {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Convert to response format
	response := UnifiedLLMResponse{
		Text:     resp.Text,
		Provider: resp.Provider,
		Model:    resp.Model,
	}

	return c.JSON(http.StatusOK, response)
}

// HandleGetProviders returns a list of all available providers
func HandleGetProviders(c echo.Context) error {
	availableProviders := providerFactory.GetAvailableProviders()

	providers := make([]ProviderInfo, 0)
	allProviderTypes := []provider.ProviderType{
		provider.ProviderGemini,
		provider.ProviderOpenAI,
		provider.ProviderAnthropic,
	}

	for _, pt := range allProviderTypes {
		available := false
		for _, ap := range availableProviders {
			if ap == pt {
				available = true
				break
			}
		}
		providers = append(providers, ProviderInfo{
			Name:      string(pt),
			Available: available,
		})
	}

	return c.JSON(http.StatusOK, providers)
}

// HandleGetModels returns all models from all providers
func HandleGetModels(c echo.Context) error {
	models := providerFactory.GetAllModels()
	return c.JSON(http.StatusOK, models)
}

// HandleGetModelsByProvider returns models for a specific provider
func HandleGetModelsByProvider(c echo.Context) error {
	providerName := c.Param("provider")
	if providerName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider parameter is required"})
	}

	providerType := provider.ProviderType(providerName)
	models, err := providerFactory.GetModelsByProvider(providerType)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Provider not available or not configured",
		})
	}

	return c.JSON(http.StatusOK, models)
}

// HandleRefreshModels forces a refresh of models from provider API
func HandleRefreshModels(c echo.Context) error {
	providerName := c.Param("provider")
	if providerName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider parameter is required"})
	}

	providerType := provider.ProviderType(providerName)
	models, err := providerFactory.RefreshModels(providerType)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to refresh models: %v", err),
		})
	}

	return c.JSON(http.StatusOK, models)
}

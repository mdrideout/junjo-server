package llm

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"junjo-server/api/llm/provider"
)

// Global cache instance (defined in provider/model_cache.go)
var globalModelCache = provider.NewModelCache(15 * time.Minute)

// HandleGetModels returns all available models across all providers
func HandleGetModels(c echo.Context) error {
	allModels := make([]provider.ModelInfo, 0)
	allModels = append(allModels, provider.OpenAIModels...)
	allModels = append(allModels, provider.AnthropicModels...)
	allModels = append(allModels, provider.GeminiModels...)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": allModels,
	})
}

// HandleGetModelsByProvider returns models for a specific provider
// Uses 3-tier strategy: cache → fetch from API → fallback to hardcoded
func HandleGetModelsByProvider(c echo.Context) error {
	providerParam := c.Param("provider")
	providerType := provider.ProviderType(providerParam)

	// Validate provider
	var hardcodedModels []provider.ModelInfo
	switch providerParam {
	case "openai":
		hardcodedModels = provider.OpenAIModels
	case "anthropic":
		hardcodedModels = provider.AnthropicModels
	case "gemini":
		hardcodedModels = provider.GeminiModels
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid provider: " + providerParam,
		})
	}

	// Tier 1: Try cache first
	if cached, ok := globalModelCache.Get(providerType); ok {
		slog.Debug("Using cached models", "provider", providerParam, "count", len(cached))
		return c.JSON(http.StatusOK, map[string]interface{}{
			"models": cached,
		})
	}

	// Tier 2: Try fetching from provider API
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	var models []provider.ModelInfo
	var fetchErr error

	switch providerParam {
	case "gemini":
		models, fetchErr = FetchGeminiModels(ctx)
	case "openai":
		models, fetchErr = FetchOpenAIModels(ctx)
	case "anthropic":
		models, fetchErr = FetchAnthropicModels(ctx)
	}

	if fetchErr == nil && len(models) > 0 {
		// Successfully fetched from API
		slog.Debug("Fetched models from provider API", "provider", providerParam, "count", len(models))
		globalModelCache.Set(providerType, models)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"models": models,
		})
	}

	// Log the error but continue with fallback
	if fetchErr != nil {
		slog.Warn("Failed to fetch models from provider API, using fallback",
			"provider", providerParam,
			"error", fetchErr.Error())
	}

	// Tier 3: Fallback to hardcoded list
	slog.Debug("Using hardcoded fallback models", "provider", providerParam, "count", len(hardcodedModels))
	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": hardcodedModels,
	})
}

// HandleRefreshModels handles refreshing models from provider API
// Forces a fresh fetch from the API, bypassing cache
func HandleRefreshModels(c echo.Context) error {
	providerParam := c.Param("provider")
	providerType := provider.ProviderType(providerParam)

	// Validate provider
	switch providerParam {
	case "openai", "anthropic", "gemini":
		// Valid provider
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid provider: " + providerParam,
		})
	}

	// Force fresh fetch from API
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	var models []provider.ModelInfo
	var err error

	switch providerParam {
	case "gemini":
		models, err = FetchGeminiModels(ctx)
	case "openai":
		models, err = FetchOpenAIModels(ctx)
	case "anthropic":
		models, err = FetchAnthropicModels(ctx)
	}

	if err != nil {
		slog.Error("Failed to refresh models from provider API",
			"provider", providerParam,
			"error", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to refresh models: " + err.Error(),
		})
	}

	// Update cache with fresh models
	globalModelCache.Set(providerType, models)
	slog.Info("Refreshed models from provider API", "provider", providerParam, "count", len(models))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": models,
	})
}

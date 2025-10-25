package llm

import (
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers the LLM service routes.
func RegisterRoutes(e *echo.Echo) {
	// Legacy endpoint - maintained for backward compatibility
	e.POST("/llm/generate", HandleGeminiTextRequest)

	// New unified endpoint with multi-provider support
	e.POST("/llm/v2/generate", HandleUnifiedLLMRequest)

	// Provider and model discovery endpoints
	e.GET("/llm/providers", HandleGetProviders)
	e.GET("/llm/models", HandleGetModels)
	e.GET("/llm/providers/:provider/models", HandleGetModelsByProvider)

	// Force refresh models from provider API
	e.POST("/llm/providers/:provider/models/refresh", HandleRefreshModels)
}

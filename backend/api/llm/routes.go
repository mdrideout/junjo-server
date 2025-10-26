package llm

import (
	"github.com/labstack/echo/v4"
	"junjo-server/api/llm/anthropic"
	"junjo-server/api/llm/gemini"
	"junjo-server/api/llm/openai"
)

// RegisterRoutes registers the LLM service routes.
func RegisterRoutes(e *echo.Echo) {
	// Provider-specific endpoints with native SDKs
	e.POST("/llm/openai/generate", openai.HandleOpenAIGenerate)
	e.POST("/llm/anthropic/generate", anthropic.HandleAnthropicGenerate)
	e.POST("/llm/gemini/generate", gemini.HandleGeminiGenerate)

	// Provider and model discovery endpoints
	e.GET("/llm/providers", HandleGetProviders)
	e.GET("/llm/models", HandleGetModels)
	e.GET("/llm/providers/:provider/models", HandleGetModelsByProvider)

	// Force refresh models from provider API
	e.POST("/llm/providers/:provider/models/refresh", HandleRefreshModels)
}

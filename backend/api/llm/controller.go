package llm

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// HandleGeminiTextRequest is the handler for the /llm/generate endpoint.
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

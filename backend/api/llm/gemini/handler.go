package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

// HandleGeminiGenerate handles requests to the Gemini generate endpoint
func HandleGeminiGenerate(c echo.Context) error {
	var req GeminiRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Error("Failed to bind request: ", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Validate required fields
	if req.Model == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "model is required"})
	}
	if len(req.Contents) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "contents are required"})
	}

	// Get API key
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "GEMINI_API_KEY environment variable is not set",
		})
	}

	// Build request body
	requestBody := map[string]interface{}{
		"contents": req.Contents,
	}

	// Add optional parameters
	if req.GenerationConfig != nil {
		requestBody["generationConfig"] = req.GenerationConfig
	}
	if req.SystemInstruction != nil {
		requestBody["system_instruction"] = req.SystemInstruction
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Create HTTP request
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		req.Model, apiKey)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Make request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.Logger().Error("Failed to generate content: ", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		c.Logger().Error("Gemini API error: ", string(respBody))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": string(respBody)})
	}

	// Parse response into our schema
	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, geminiResp)
}

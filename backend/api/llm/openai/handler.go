package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// HandleOpenAIGenerate handles requests to the OpenAI generate endpoint
func HandleOpenAIGenerate(c echo.Context) error {
	var req OpenAIRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Error("Failed to bind request: ", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Validate required fields
	if req.Model == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "model is required"})
	}
	if len(req.Messages) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "messages are required"})
	}

	// Get API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "OPENAI_API_KEY environment variable is not set",
		})
	}

	// Build request body (matches OpenAI API exactly)
	requestBody := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
	}

	// Add optional parameters
	if req.Temperature != nil {
		requestBody["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		requestBody["max_tokens"] = *req.MaxTokens
	}
	if req.MaxCompletionTokens != nil {
		requestBody["max_completion_tokens"] = *req.MaxCompletionTokens
	}
	if req.ReasoningEffort != nil {
		requestBody["reasoning_effort"] = *req.ReasoningEffort
	}
	if req.TopP != nil {
		requestBody["top_p"] = *req.TopP
	}
	if len(req.StopSequences) > 0 {
		requestBody["stop"] = req.StopSequences
	}

	// Add response format for JSON mode
	if req.ResponseFormat != nil && req.ResponseFormat.Type == "json_object" {
		requestBody["response_format"] = map[string]string{
			"type": "json_object",
		}
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Set headers
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
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
		c.Logger().Error("OpenAI API error: ", string(respBody))
		if strings.Contains(string(respBody), "api key") {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": string(respBody)})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": string(respBody)})
	}

	// Parse response into our schema
	var openaiResp OpenAIResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, openaiResp)
}

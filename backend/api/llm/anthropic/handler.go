package anthropic

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// HandleAnthropicGenerate handles requests to the Anthropic generate endpoint
func HandleAnthropicGenerate(c echo.Context) error {
	var req AnthropicRequest
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
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096 // Default
	}

	// Get API key
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "ANTHROPIC_API_KEY environment variable is not set",
		})
	}

	// Build request body (matches Anthropic API exactly)
	requestBody := map[string]interface{}{
		"model":      req.Model,
		"messages":   req.Messages,
		"max_tokens": req.MaxTokens,
	}

	// Add system prompt if provided
	if req.System != "" {
		requestBody["system"] = req.System
	}

	// Add optional parameters
	if req.Temperature != nil {
		requestBody["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		requestBody["top_p"] = *req.TopP
	}
	if req.TopK != nil {
		requestBody["top_k"] = *req.TopK
	}
	if len(req.StopSequences) > 0 {
		requestBody["stop_sequences"] = req.StopSequences
	}
	if req.Thinking != nil {
		requestBody["thinking"] = map[string]interface{}{
			"type":          req.Thinking.Type,
			"budget_tokens": req.Thinking.BudgetTokens,
		}
	}

	// Handle JSON mode using tool calling (Anthropic's recommended approach)
	if req.JSONMode {
		requestBody["tools"] = []map[string]interface{}{
			{
				"name":        "structured_output",
				"description": "Return data in structured JSON format",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"output": map[string]interface{}{
							"type":        "object",
							"description": "The structured JSON response",
						},
					},
					"required": []string{"output"},
				},
			},
		}
		requestBody["tool_choice"] = map[string]string{"type": "any"}
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Set headers (Anthropic-specific)
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

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
		c.Logger().Error("Anthropic API error: ", string(respBody))
		if strings.Contains(string(respBody), "api key") {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": string(respBody)})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": string(respBody)})
	}

	// Parse response into our schema
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, anthropicResp)
}

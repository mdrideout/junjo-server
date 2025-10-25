package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const geminiAPIBaseURL = "https://generativelanguage.googleapis.com/v1beta/models/"

// GeminiProvider implements the LLMProvider interface for Google Gemini
// Uses direct HTTP calls to Gemini API (not Bellman)
type GeminiProvider struct {
	apiKey string
}

// NewGeminiProvider creates a new Gemini provider instance
func NewGeminiProvider(apiKey string) *GeminiProvider {
	return &GeminiProvider{
		apiKey: apiKey,
	}
}

// GenerateContent generates content using Gemini
func (p *GeminiProvider) GenerateContent(ctx context.Context, req *UnifiedRequest) (*UnifiedResponse, error) {
	if err := p.ValidateAPIKey(); err != nil {
		return nil, err
	}

	// Build Gemini request
	geminiReq := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": req.Prompt},
				},
			},
		},
	}

	// Add generation config if needed
	genConfig := make(map[string]interface{})
	if req.JSONMode {
		genConfig["responseMimeType"] = "application/json"
	}
	if req.Temperature > 0 {
		genConfig["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		genConfig["maxOutputTokens"] = req.MaxTokens
	}
	if req.TopP > 0 {
		genConfig["topP"] = req.TopP
	}
	if req.TopK > 0 {
		genConfig["topK"] = req.TopK
	}
	if len(req.StopSequences) > 0 {
		genConfig["stopSequences"] = req.StopSequences
	}
	if len(genConfig) > 0 {
		geminiReq["generationConfig"] = genConfig
	}

	// Add system instruction if provided
	if req.SystemPrompt != "" {
		geminiReq["system_instruction"] = map[string]interface{}{
			"parts": []map[string]string{
				{"text": req.SystemPrompt},
			},
		}
	}

	// Marshal request
	reqBody, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	url := fmt.Sprintf("%s%s:generateContent?key=%s", geminiAPIBaseURL, req.Model, p.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini generation failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var geminiResp struct {
		Candidates []struct{
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	text := geminiResp.Candidates[0].Content.Parts[0].Text

	return &UnifiedResponse{
		Text:     text,
		Provider: string(ProviderGemini),
		Model:    req.Model,
	}, nil
}

// SupportedModels returns the list of supported Gemini models
func (p *GeminiProvider) SupportedModels() []ModelInfo {
	return GeminiModels
}

// FetchAvailableModels fetches the latest model list from the Gemini API
func (p *GeminiProvider) FetchAvailableModels(ctx context.Context) ([]ModelInfo, error) {
	if err := p.ValidateAPIKey(); err != nil {
		return nil, err
	}

	// Call Gemini models API
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", p.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Gemini models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var geminiResp struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			Description                string   `json:"description"`
			InputTokenLimit            int      `json:"inputTokenLimit"`
			OutputTokenLimit           int      `json:"outputTokenLimit"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini models response: %w", err)
	}

	// Convert to ModelInfo and filter
	models := make([]ModelInfo, 0, len(geminiResp.Models))
	for _, m := range geminiResp.Models {
		// Filter to only models that support generateContent
		supportsGenerate := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsGenerate = true
				break
			}
		}
		if !supportsGenerate {
			continue
		}

		// Extract model ID from name (e.g., "models/gemini-2.5-flash" -> "gemini-2.5-flash")
		modelID := m.Name
		if len(m.Name) > 7 && m.Name[:7] == "models/" {
			modelID = m.Name[7:]
		}

		models = append(models, ModelInfo{
			ID:              modelID,
			Name:            m.DisplayName,
			Provider:        string(ProviderGemini),
			Description:     m.Description,
			ContextWindow:   m.InputTokenLimit,
			MaxOutputTokens: m.OutputTokenLimit,
		})
	}

	return models, nil
}

// ValidateAPIKey checks if the API key is available
func (p *GeminiProvider) ValidateAPIKey() error {
	if p.apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}
	return nil
}

// GetBellmanClient returns nil as Gemini provider doesn't use Bellman
func (p *GeminiProvider) GetBellmanClient() interface{} {
	return nil
}

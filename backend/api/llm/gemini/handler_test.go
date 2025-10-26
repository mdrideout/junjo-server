package gemini

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGeminiGenerationConfig_WithJSONSchema verifies JSON schema in generation config
func TestGeminiGenerationConfig_WithJSONSchema(t *testing.T) {
	// JSON Schema format (lowercase types) - same as OpenAI/Anthropic
	// Modern Gemini response_json_schema uses JSON Schema, not OpenAPI 3.0
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "The title of the article",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The main content",
			},
			"tags": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"published": map[string]interface{}{
				"type": "boolean",
			},
		},
		"required": []string{"title", "content"},
	}

	temperature := 0.8
	maxOutputTokens := 2048

	generationConfig := GeminiGenerationConfig{
		Temperature:        &temperature,
		MaxOutputTokens:    &maxOutputTokens,
		ResponseMimeType:   "application/json",
		ResponseJSONSchema: schema,
	}

	// Marshal to JSON
	data, err := json.Marshal(generationConfig)
	assert.NoError(t, err, "Should marshal without error")

	// Unmarshal back to verify structure
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err, "Should unmarshal without error")

	// Verify fields
	assert.Equal(t, 0.8, unmarshaled["temperature"])
	assert.Equal(t, float64(2048), unmarshaled["maxOutputTokens"])
	assert.Equal(t, "application/json", unmarshaled["responseMimeType"])
	assert.NotNil(t, unmarshaled["response_json_schema"])

	schemaMap := unmarshaled["response_json_schema"].(map[string]interface{})
	assert.Equal(t, "object", schemaMap["type"])
	assert.NotNil(t, schemaMap["properties"])
	assert.NotNil(t, schemaMap["required"])
}

// TestGeminiGenerationConfig_BasicJSONMode verifies basic JSON mode without schema
func TestGeminiGenerationConfig_BasicJSONMode(t *testing.T) {
	temperature := 0.7

	generationConfig := GeminiGenerationConfig{
		Temperature:      &temperature,
		ResponseMimeType: "application/json",
		// No ResponseJSONSchema - basic JSON mode
	}

	data, err := json.Marshal(generationConfig)
	assert.NoError(t, err)

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, 0.7, unmarshaled["temperature"])
	assert.Equal(t, "application/json", unmarshaled["responseMimeType"])
	assert.Nil(t, unmarshaled["response_json_schema"]) // Should not have schema
}

// TestGeminiRequest_WithJSONSchema verifies full request with JSON schema
func TestGeminiRequest_WithJSONSchema(t *testing.T) {
	// JSON Schema format (lowercase types)
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Person's name",
			},
			"age": map[string]interface{}{
				"type":        "integer",
				"description": "Person's age",
			},
			"email": map[string]interface{}{
				"type":   "string",
				"format": "email",
			},
		},
		"required": []string{"name", "age"},
	}

	temperature := 0.5
	maxOutputTokens := 1024

	request := GeminiRequest{
		Model: "gemini-2.5-flash",
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: "Extract person information from this text."},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:        &temperature,
			MaxOutputTokens:    &maxOutputTokens,
			ResponseMimeType:   "application/json",
			ResponseJSONSchema: schema,
		},
	}

	// Marshal to JSON (simulating what gets sent to Gemini API)
	data, err := json.Marshal(request)
	assert.NoError(t, err)

	// Unmarshal to verify structure
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)

	// Verify basic fields
	assert.Equal(t, "gemini-2.5-flash", unmarshaled["model"])
	assert.NotNil(t, unmarshaled["contents"])

	// Verify generationConfig
	genConfigMap := unmarshaled["generationConfig"].(map[string]interface{})
	assert.Equal(t, 0.5, genConfigMap["temperature"])
	assert.Equal(t, float64(1024), genConfigMap["maxOutputTokens"])
	assert.Equal(t, "application/json", genConfigMap["responseMimeType"])

	// Verify response_json_schema
	schemaMap := genConfigMap["response_json_schema"].(map[string]interface{})
	assert.Equal(t, "object", schemaMap["type"])
	assert.NotNil(t, schemaMap["properties"])

	properties := schemaMap["properties"].(map[string]interface{})
	assert.NotNil(t, properties["name"])
	assert.NotNil(t, properties["age"])
	assert.NotNil(t, properties["email"])

	required := schemaMap["required"].([]interface{})
	assert.Contains(t, required, "name")
	assert.Contains(t, required, "age")
}

// TestGeminiRequest_WithThinkingAndSchema verifies schema works alongside thinking config
func TestGeminiRequest_WithThinkingAndSchema(t *testing.T) {
	// JSON Schema format (lowercase types)
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"analysis": map[string]interface{}{
				"type": "string",
			},
		},
	}

	temperature := 0.7
	thinkingBudget := 1024
	includeThoughts := true

	request := GeminiRequest{
		Model: "gemini-2.5-pro",
		Contents: []GeminiContent{
			{Parts: []GeminiPart{{Text: "Analyze this deeply."}}},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:        &temperature,
			ResponseMimeType:   "application/json",
			ResponseJSONSchema: schema,
			ThinkingConfig: &GeminiThinkingConfig{
				ThinkingBudget:  &thinkingBudget,
				IncludeThoughts: &includeThoughts,
			},
		},
	}

	data, err := json.Marshal(request)
	assert.NoError(t, err)

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)

	genConfigMap := unmarshaled["generationConfig"].(map[string]interface{})

	// Verify both schema and thinking config are present
	assert.NotNil(t, genConfigMap["response_json_schema"])
	assert.NotNil(t, genConfigMap["thinkingConfig"])

	thinkingConfigMap := genConfigMap["thinkingConfig"].(map[string]interface{})
	assert.Equal(t, float64(1024), thinkingConfigMap["thinkingBudget"])
	assert.Equal(t, true, thinkingConfigMap["includeThoughts"])

	schemaMap := genConfigMap["response_json_schema"].(map[string]interface{})
	assert.Equal(t, "object", schemaMap["type"])
}

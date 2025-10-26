package openai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestOpenAIJSONSchema_Marshaling verifies JSON schema structure is correctly marshaled
func TestOpenAIJSONSchema_Marshaling(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The name of the person",
			},
			"age": map[string]interface{}{
				"type":        "number",
				"description": "The age of the person",
			},
		},
		"required":             []string{"name", "age"},
		"additionalProperties": false,
	}

	strict := true
	jsonSchema := OpenAIJSONSchema{
		Name:   "person_schema",
		Strict: &strict,
		Schema: schema,
	}

	// Marshal to JSON
	data, err := json.Marshal(jsonSchema)
	assert.NoError(t, err, "Should marshal without error")

	// Unmarshal back to verify structure
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err, "Should unmarshal without error")

	// Verify fields
	assert.Equal(t, "person_schema", unmarshaled["name"])
	assert.Equal(t, true, unmarshaled["strict"])
	assert.NotNil(t, unmarshaled["schema"])

	schemaMap := unmarshaled["schema"].(map[string]interface{})
	assert.Equal(t, "object", schemaMap["type"])
	assert.NotNil(t, schemaMap["properties"])
}

// TestOpenAIResponseFormat_JSONObject verifies basic JSON object response format
func TestOpenAIResponseFormat_JSONObject(t *testing.T) {
	responseFormat := OpenAIResponseFormat{
		Type: "json_object",
	}

	data, err := json.Marshal(responseFormat)
	assert.NoError(t, err)

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, "json_object", unmarshaled["type"])
	assert.Nil(t, unmarshaled["json_schema"]) // Should not have json_schema field
}

// TestOpenAIResponseFormat_JSONSchema verifies structured output response format
func TestOpenAIResponseFormat_JSONSchema(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type": "string",
			},
			"content": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"title"},
	}

	strict := true
	responseFormat := OpenAIResponseFormat{
		Type: "json_schema",
		JSONSchema: &OpenAIJSONSchema{
			Name:   "article_schema",
			Strict: &strict,
			Schema: schema,
		},
	}

	data, err := json.Marshal(responseFormat)
	assert.NoError(t, err)

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, "json_schema", unmarshaled["type"])
	assert.NotNil(t, unmarshaled["json_schema"])

	jsonSchemaMap := unmarshaled["json_schema"].(map[string]interface{})
	assert.Equal(t, "article_schema", jsonSchemaMap["name"])
	assert.Equal(t, true, jsonSchemaMap["strict"])
	assert.NotNil(t, jsonSchemaMap["schema"])
}

// TestOpenAIRequest_WithJSONSchema verifies full request with JSON schema
func TestOpenAIRequest_WithJSONSchema(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "A brief summary",
			},
			"keywords": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required":             []string{"summary"},
		"additionalProperties": false,
	}

	strict := true
	temperature := 0.7
	maxTokens := 2048

	request := OpenAIRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Extract structured data from this text."},
		},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
		ResponseFormat: &OpenAIResponseFormat{
			Type: "json_schema",
			JSONSchema: &OpenAIJSONSchema{
				Name:   "extraction_schema",
				Strict: &strict,
				Schema: schema,
			},
		},
	}

	// Marshal to JSON (simulating what gets sent to OpenAI API)
	data, err := json.Marshal(request)
	assert.NoError(t, err)

	// Unmarshal to verify structure
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)

	// Verify basic fields
	assert.Equal(t, "gpt-4", unmarshaled["model"])
	assert.Equal(t, 0.7, unmarshaled["temperature"])
	assert.Equal(t, float64(2048), unmarshaled["max_tokens"])

	// Verify response_format structure
	responseFormatMap := unmarshaled["response_format"].(map[string]interface{})
	assert.Equal(t, "json_schema", responseFormatMap["type"])

	jsonSchemaMap := responseFormatMap["json_schema"].(map[string]interface{})
	assert.Equal(t, "extraction_schema", jsonSchemaMap["name"])
	assert.Equal(t, true, jsonSchemaMap["strict"])

	schemaMap := jsonSchemaMap["schema"].(map[string]interface{})
	assert.Equal(t, "object", schemaMap["type"])
	assert.NotNil(t, schemaMap["properties"])

	properties := schemaMap["properties"].(map[string]interface{})
	assert.NotNil(t, properties["summary"])
	assert.NotNil(t, properties["keywords"])
}

// TestOpenAIRequest_Compatibility verifies backward compatibility with basic JSON mode
func TestOpenAIRequest_Compatibility(t *testing.T) {
	temperature := 0.5

	// Old-style basic JSON mode request
	requestBasic := OpenAIRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Return JSON"},
		},
		Temperature: &temperature,
		ResponseFormat: &OpenAIResponseFormat{
			Type: "json_object",
		},
	}

	data, err := json.Marshal(requestBasic)
	assert.NoError(t, err)

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)

	responseFormat := unmarshaled["response_format"].(map[string]interface{})
	assert.Equal(t, "json_object", responseFormat["type"])
	assert.Nil(t, responseFormat["json_schema"]) // Should not have schema for basic mode
}

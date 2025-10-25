package provider

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestModelCache_SetAndGet verifies basic cache set and get operations
func TestModelCache_SetAndGet(t *testing.T) {
	cache := NewModelCache(15 * time.Minute)

	models := []ModelInfo{
		{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Provider: "openai"},
	}

	// Set models in cache
	cache.Set(ProviderOpenAI, models)

	// Retrieve models from cache
	retrieved, exists := cache.Get(ProviderOpenAI)
	assert.True(t, exists, "Models should exist in cache")
	assert.Equal(t, models, retrieved, "Retrieved models should match stored models")
}

// TestModelCache_GetMiss verifies cache miss behavior
func TestModelCache_GetMiss(t *testing.T) {
	cache := NewModelCache(15 * time.Minute)

	// Try to get models for provider that was never set
	retrieved, exists := cache.Get(ProviderGemini)
	assert.False(t, exists, "Should return false for cache miss")
	assert.Nil(t, retrieved, "Should return nil models for cache miss")
}

// TestModelCache_TTLExpiration verifies cache entries expire after TTL
func TestModelCache_TTLExpiration(t *testing.T) {
	// Use very short TTL for testing
	cache := NewModelCache(100 * time.Millisecond)

	models := []ModelInfo{
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: "gemini"},
	}

	cache.Set(ProviderGemini, models)

	// Should exist immediately
	_, exists := cache.Get(ProviderGemini)
	assert.True(t, exists, "Models should exist immediately after set")

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	_, exists = cache.Get(ProviderGemini)
	assert.False(t, exists, "Models should be expired after TTL")
}

// TestModelCache_MultipleProviders verifies cache correctly isolates models by provider
func TestModelCache_MultipleProviders(t *testing.T) {
	cache := NewModelCache(15 * time.Minute)

	geminiModels := []ModelInfo{
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: "gemini"},
	}
	openaiModels := []ModelInfo{
		{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
	}

	cache.Set(ProviderGemini, geminiModels)
	cache.Set(ProviderOpenAI, openaiModels)

	// Retrieve Gemini models
	retrievedGemini, exists := cache.Get(ProviderGemini)
	assert.True(t, exists, "Gemini models should exist")
	assert.Equal(t, geminiModels, retrievedGemini, "Should get correct Gemini models")

	// Retrieve OpenAI models
	retrievedOpenAI, exists := cache.Get(ProviderOpenAI)
	assert.True(t, exists, "OpenAI models should exist")
	assert.Equal(t, openaiModels, retrievedOpenAI, "Should get correct OpenAI models")
}

// TestModelCache_Clear verifies clearing all cache entries
func TestModelCache_Clear(t *testing.T) {
	cache := NewModelCache(15 * time.Minute)

	models := []ModelInfo{
		{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
	}

	cache.Set(ProviderOpenAI, models)
	cache.Set(ProviderGemini, models)

	// Verify entries exist
	_, exists := cache.Get(ProviderOpenAI)
	assert.True(t, exists, "OpenAI models should exist before clear")

	// Clear cache
	cache.Clear()

	// Verify all entries are gone
	_, exists = cache.Get(ProviderOpenAI)
	assert.False(t, exists, "OpenAI models should not exist after clear")
	_, exists = cache.Get(ProviderGemini)
	assert.False(t, exists, "Gemini models should not exist after clear")
}

// TestModelCache_ClearProvider verifies clearing a specific provider's cache
func TestModelCache_ClearProvider(t *testing.T) {
	cache := NewModelCache(15 * time.Minute)

	geminiModels := []ModelInfo{
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: "gemini"},
	}
	openaiModels := []ModelInfo{
		{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
	}

	cache.Set(ProviderGemini, geminiModels)
	cache.Set(ProviderOpenAI, openaiModels)

	// Clear only Gemini
	cache.ClearProvider(ProviderGemini)

	// Verify Gemini is cleared
	_, exists := cache.Get(ProviderGemini)
	assert.False(t, exists, "Gemini models should not exist after clear")

	// Verify OpenAI still exists
	retrieved, exists := cache.Get(ProviderOpenAI)
	assert.True(t, exists, "OpenAI models should still exist")
	assert.Equal(t, openaiModels, retrieved, "OpenAI models should be unchanged")
}

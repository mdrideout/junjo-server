# Phase 3: Dynamic Model Fetching - Testing Plan

**Status:** Not Yet Implemented
**Related:** PHASE3_DYNAMIC_MODEL_FETCHING.md
**Created:** 2025-01-25

---

## Overview

This document outlines the testing strategy for the Phase 3 Dynamic Model Fetching feature. Tests should be implemented after the main feature is complete and stable.

---

## Unit Tests

### 1. Model Cache Tests

**File:** `backend/api/llm/provider/model_cache_test.go`

#### Test Cases:
- **TestModelCache_SetAndGet**: Verify cache stores and retrieves models correctly
- **TestModelCache_GetMiss**: Verify Get() returns false when key doesn't exist
- **TestModelCache_TTLExpiration**: Verify cached models expire after 15 minutes
- **TestModelCache_ConcurrentAccess**: Verify thread-safe access with multiple goroutines
- **TestModelCache_MultipleProviders**: Verify cache correctly isolates models by provider

```go
func TestModelCache_SetAndGet(t *testing.T) {
    cache := NewModelCache(15 * time.Minute)

    models := []ModelInfo{
        {ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
    }

    cache.Set(ProviderOpenAI, models)

    retrieved, exists := cache.Get(ProviderOpenAI)
    assert.True(t, exists)
    assert.Equal(t, models, retrieved)
}

func TestModelCache_TTLExpiration(t *testing.T) {
    cache := NewModelCache(100 * time.Millisecond)

    models := []ModelInfo{
        {ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
    }

    cache.Set(ProviderOpenAI, models)

    // Should exist immediately
    _, exists := cache.Get(ProviderOpenAI)
    assert.True(t, exists)

    // Wait for TTL to expire
    time.Sleep(150 * time.Millisecond)

    // Should be expired
    _, exists = cache.Get(ProviderOpenAI)
    assert.False(t, exists)
}

func TestModelCache_ConcurrentAccess(t *testing.T) {
    cache := NewModelCache(15 * time.Minute)
    var wg sync.WaitGroup

    // Multiple goroutines writing
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            models := []ModelInfo{
                {ID: fmt.Sprintf("model-%d", id), Provider: "test"},
            }
            cache.Set(ProviderOpenAI, models)
        }(i)
    }

    // Multiple goroutines reading
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            cache.Get(ProviderOpenAI)
        }()
    }

    wg.Wait()
}
```

---

### 2. Provider FetchAvailableModels Tests

#### Gemini Provider Tests

**File:** `backend/api/llm/provider/gemini_provider_test.go`

**Test Cases:**
- **TestGeminiProvider_FetchAvailableModels_Success**: Mock successful API response
- **TestGeminiProvider_FetchAvailableModels_NetworkError**: Simulate network failure
- **TestGeminiProvider_FetchAvailableModels_InvalidResponse**: Malformed JSON response
- **TestGeminiProvider_FetchAvailableModels_FilterNonGenerativeModels**: Verify filtering logic

```go
func TestGeminiProvider_FetchAvailableModels_Success(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        response := map[string]interface{}{
            "models": []map[string]interface{}{
                {
                    "name":                      "models/gemini-2.5-flash",
                    "displayName":               "Gemini 2.5 Flash",
                    "description":               "Fast model",
                    "inputTokenLimit":           1000000,
                    "outputTokenLimit":          8192,
                    "supportedGenerationMethods": []string{"generateContent"},
                },
                {
                    "name":                      "models/embedding-001",
                    "displayName":               "Embedding Model",
                    "supportedGenerationMethods": []string{"embedContent"},
                },
            },
        }
        json.NewEncoder(w).Encode(response)
    }))
    defer server.Close()

    provider := NewGeminiProvider("test-key")
    // Override API URL for testing
    provider.apiBaseURL = server.URL

    ctx := context.Background()
    models, err := provider.FetchAvailableModels(ctx)

    assert.NoError(t, err)
    assert.Len(t, models, 1) // Only generative model, embedding filtered out
    assert.Equal(t, "gemini-2.5-flash", models[0].ID)
}
```

#### OpenAI Provider Tests

**File:** `backend/api/llm/provider/openai_provider_test.go`

**Test Cases:**
- **TestOpenAIProvider_FetchAvailableModels_Success**: Mock successful API response
- **TestOpenAIProvider_FetchAvailableModels_FilterNonChatModels**: Verify gpt-* filtering
- **TestOpenAIProvider_FetchAvailableModels_AuthError**: Test 401 response handling

```go
func TestOpenAIProvider_FetchAvailableModels_FilterNonChatModels(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        response := map[string]interface{}{
            "data": []map[string]interface{}{
                {"id": "gpt-4", "created": 1234567890},
                {"id": "gpt-3.5-turbo", "created": 1234567891},
                {"id": "text-embedding-ada-002", "created": 1234567892},
                {"id": "whisper-1", "created": 1234567893},
                {"id": "dall-e-3", "created": 1234567894},
                {"id": "tts-1", "created": 1234567895},
            },
        }
        json.NewEncoder(w).Encode(response)
    }))
    defer server.Close()

    provider := NewOpenAIProvider("test-key")
    provider.apiBaseURL = server.URL

    ctx := context.Background()
    models, err := provider.FetchAvailableModels(ctx)

    assert.NoError(t, err)
    assert.Len(t, models, 2) // Only gpt-4 and gpt-3.5-turbo
}
```

#### Anthropic Provider Tests

**File:** `backend/api/llm/provider/anthropic_provider_test.go`

**Test Cases:**
- **TestAnthropicProvider_FetchAvailableModels_Success**: Mock successful API response
- **TestAnthropicProvider_FetchAvailableModels_Pagination**: Test pagination handling
- **TestAnthropicProvider_FetchAvailableModels_MissingAPIKey**: Test error when API key missing

---

### 3. ProviderFactory Tests

**File:** `backend/api/llm/provider/provider_test.go`

**Test Cases:**
- **TestProviderFactory_GetModelsByProvider_CacheHit**: Verify cache is used on second call
- **TestProviderFactory_GetModelsByProvider_CacheMiss**: Verify API fetch on first call
- **TestProviderFactory_GetModelsByProvider_FallbackToHardcoded**: Test fallback when API fails
- **TestProviderFactory_RefreshModels**: Verify force refresh bypasses cache

```go
func TestProviderFactory_GetModelsByProvider_CacheHit(t *testing.T) {
    // Mock provider
    mockProvider := &MockProvider{
        fetchCount: 0,
        models: []ModelInfo{
            {ID: "test-model", Name: "Test Model", Provider: "test"},
        },
    }

    factory := &ProviderFactory{
        providers: map[ProviderType]LLMProvider{
            ProviderOpenAI: mockProvider,
        },
    }

    // First call - should fetch
    models1, err := factory.GetModelsByProvider(ProviderOpenAI)
    assert.NoError(t, err)
    assert.Equal(t, 1, mockProvider.fetchCount)

    // Second call - should use cache
    models2, err := factory.GetModelsByProvider(ProviderOpenAI)
    assert.NoError(t, err)
    assert.Equal(t, 1, mockProvider.fetchCount) // No additional fetch
    assert.Equal(t, models1, models2)
}

func TestProviderFactory_GetModelsByProvider_FallbackToHardcoded(t *testing.T) {
    mockProvider := &MockProvider{
        fetchError: errors.New("API unavailable"),
        hardcodedModels: []ModelInfo{
            {ID: "fallback-model", Name: "Fallback Model", Provider: "test"},
        },
    }

    factory := &ProviderFactory{
        providers: map[ProviderType]LLMProvider{
            ProviderOpenAI: mockProvider,
        },
    }

    models, err := factory.GetModelsByProvider(ProviderOpenAI)
    assert.NoError(t, err)
    assert.Equal(t, mockProvider.hardcodedModels, models)
}
```

---

## Integration Tests

### 1. API Endpoint Tests

**File:** `backend/api/llm/controller_test.go`

**Test Cases:**
- **TestGetModelsByProvider_Success**: GET /llm/providers/openai/models returns models
- **TestGetModelsByProvider_CachedResponse**: Verify caching behavior
- **TestGetModelsByProvider_InvalidProvider**: 404 for unknown provider
- **TestRefreshModels_Success**: POST /llm/providers/openai/models/refresh works
- **TestRefreshModels_BypassesCache**: Verify refresh ignores cache

```go
func TestGetModelsByProvider_Success(t *testing.T) {
    e := echo.New()
    req := httptest.NewRequest(http.MethodGet, "/llm/providers/openai/models", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    c.SetParamNames("provider")
    c.SetParamValues("openai")

    err := HandleGetModelsByProvider(c)

    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, rec.Code)

    var models []ModelInfo
    json.Unmarshal(rec.Body.Bytes(), &models)
    assert.NotEmpty(t, models)
}

func TestRefreshModels_Success(t *testing.T) {
    e := echo.New()
    req := httptest.NewRequest(http.MethodPost, "/llm/providers/openai/models/refresh", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    c.SetParamNames("provider")
    c.SetParamValues("openai")

    err := HandleRefreshModels(c)

    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, rec.Code)
}
```

---

## Frontend Tests

### 1. Component Tests

**File:** `frontend/src/features/prompt-playground/components/ModelSelector.test.tsx`

**Test Cases:**
- **Test refresh button click**: Verify refresh function is called
- **Test loading state**: Verify spinner shows during refresh
- **Test model metadata display**: Verify description and context window are shown
- **Test refresh error handling**: Verify error state

```typescript
describe('ModelSelector', () => {
  test('refresh button triggers model refresh', async () => {
    const mockRefresh = jest.fn()
    global.fetch = mockRefresh

    render(<ModelSelector provider="openai" originalModel="gpt-4" />)

    const refreshButton = screen.getByTitle('Refresh models from provider API')
    fireEvent.click(refreshButton)

    expect(mockRefresh).toHaveBeenCalledWith(
      expect.stringContaining('/llm/providers/openai/models/refresh'),
      expect.any(Object)
    )
  })

  test('displays loading spinner during refresh', async () => {
    render(<ModelSelector provider="openai" originalModel="gpt-4" />)

    const refreshButton = screen.getByTitle('Refresh models from provider API')
    fireEvent.click(refreshButton)

    expect(screen.getByTestId('reload-icon')).toHaveClass('animate-spin')
  })
})
```

---

## Manual Testing Checklist

### Setup
- [ ] Ensure GEMINI_API_KEY is set
- [ ] Ensure OPENAI_API_KEY is set
- [ ] Ensure ANTHROPIC_API_KEY is set

### Backend Testing
- [ ] GET /llm/providers/gemini/models returns models with metadata
- [ ] GET /llm/providers/openai/models returns only gpt-* models
- [ ] GET /llm/providers/anthropic/models returns Claude models
- [ ] Second call to GET endpoint uses cache (check logs for "cache hit")
- [ ] POST /llm/providers/:provider/models/refresh forces new API call
- [ ] API failures fallback to hardcoded lists (disconnect network and test)
- [ ] Cache expires after 15 minutes (wait and verify new API call)

### Frontend Testing
- [ ] Open prompt playground
- [ ] Select OpenAI provider
- [ ] Refresh button appears next to model selector
- [ ] Click refresh button - spinner shows during loading
- [ ] Model list updates after refresh
- [ ] Model dropdown shows descriptions and context window
- [ ] Select Gemini provider and refresh
- [ ] Verify only generative models appear
- [ ] Select Anthropic provider and refresh
- [ ] Test with network disconnected - should show fallback models

### Performance Testing
- [ ] First API call takes 1-3 seconds (acceptable)
- [ ] Cached calls return instantly
- [ ] UI remains responsive during refresh
- [ ] No race conditions with rapid provider switching

### Error Scenarios
- [ ] Invalid API key returns graceful error
- [ ] Network timeout falls back to hardcoded list
- [ ] Malformed API response falls back to hardcoded list
- [ ] Provider with no API key shows hardcoded list

---

## Test Data

### Mock Gemini API Response
```json
{
  "models": [
    {
      "name": "models/gemini-2.5-flash",
      "displayName": "Gemini 2.5 Flash",
      "description": "Fast and efficient model",
      "inputTokenLimit": 1000000,
      "outputTokenLimit": 8192,
      "supportedGenerationMethods": ["generateContent"]
    }
  ]
}
```

### Mock OpenAI API Response
```json
{
  "data": [
    {
      "id": "gpt-4",
      "object": "model",
      "created": 1687882411,
      "owned_by": "openai"
    },
    {
      "id": "text-embedding-ada-002",
      "object": "model",
      "created": 1671217299,
      "owned_by": "openai-internal"
    }
  ]
}
```

### Mock Anthropic API Response
```json
{
  "data": [
    {
      "id": "claude-3-opus-20240229",
      "display_name": "Claude 3 Opus",
      "created_at": "2024-02-29T00:00:00Z",
      "type": "model"
    }
  ],
  "has_more": false
}
```

---

## Success Criteria

- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] All frontend tests pass
- [ ] Manual testing checklist completed
- [ ] Code coverage > 80% for new code
- [ ] No performance regression
- [ ] Logging is consistent with slog patterns

---

## Notes

- Use `httptest.NewServer()` for mocking HTTP APIs in Go tests
- Use `jest.fn()` for mocking fetch in React tests
- Consider using `testify/assert` for cleaner assertions
- Consider using `@testing-library/react` for frontend tests

---

**Last Updated:** 2025-01-25

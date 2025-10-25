# Phase 3: Dynamic Model Fetching Enhancement

## Overview

This document outlines a future enhancement to fetch LLM model lists dynamically from provider APIs instead of maintaining hardcoded lists. This will keep the playground up-to-date with the latest models without requiring code changes.

**Status:** Not yet implemented
**Priority:** Medium (Enhancement, not critical)
**Complexity:** Medium
**Estimated Effort:** 4-6 hours

---

## Current State

### Limitations

1. **Hardcoded Model Lists** in `backend/api/llm/provider/models.go`:
   - Gemini: 5 models
   - OpenAI: 5 models
   - Anthropic: 5 models
   - Lists become outdated as providers release new models
   - Requires code changes to add support for new models

2. **No Model Metadata**:
   - Context window size
   - Release date
   - Capabilities (vision, function calling, etc.)
   - Pricing information

3. **Manual Maintenance**:
   - Developer must monitor provider announcements
   - Update code and deploy to add new models

---

## Solution Design

### Provider API Endpoints

All three providers have public APIs to list available models:

#### Gemini (Google Generative Language API)
```
GET https://generativelanguage.googleapis.com/v1beta/models?key=${API_KEY}
```

**Response includes:**
- Model name
- Display name
- Description
- Version
- Input/output token limits
- Supported generation methods

#### OpenAI
```
GET https://api.openai.com/v1/models
Headers: Authorization: Bearer ${API_KEY}
```

**Response includes:**
- Model ID
- Owner
- Created timestamp
- Object type

#### Anthropic
```
GET https://api.anthropic.com/v1/models
Headers:
  x-api-key: ${API_KEY}
  anthropic-version: 2023-06-01
```

**Response includes:**
- Model ID
- Display name
- Created timestamp
- Type
- Pagination support

---

## Implementation Plan

### Backend Changes

#### 1. Update Provider Interface

**File:** `backend/api/llm/provider/provider.go`

```go
type LLMProvider interface {
    // ... existing methods ...

    // FetchAvailableModels fetches the latest model list from the provider API
    FetchAvailableModels(ctx context.Context) ([]ModelInfo, error)
}

// Enhanced ModelInfo with additional metadata
type ModelInfo struct {
    ID              string            `json:"id"`
    Name            string            `json:"name"`
    Provider        string            `json:"provider"`
    Description     string            `json:"description,omitempty"`
    ContextWindow   int               `json:"contextWindow,omitempty"`   // NEW
    MaxOutputTokens int               `json:"maxOutputTokens,omitempty"` // NEW
    CreatedAt       string            `json:"createdAt,omitempty"`       // NEW
    Capabilities    []string          `json:"capabilities,omitempty"`    // NEW (e.g., "vision", "function-calling")
    Metadata        map[string]string `json:"metadata,omitempty"`        // NEW (provider-specific)
}
```

#### 2. Implement Provider-Specific Fetchers

**File:** `backend/api/llm/provider/gemini_provider.go`

```go
func (p *GeminiProvider) FetchAvailableModels(ctx context.Context) ([]ModelInfo, error) {
    url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", p.apiKey)

    resp, err := http.Get(url)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch Gemini models: %w", err)
    }
    defer resp.Body.Close()

    var geminiResp struct {
        Models []struct {
            Name               string   `json:"name"`
            DisplayName        string   `json:"displayName"`
            Description        string   `json:"description"`
            InputTokenLimit    int      `json:"inputTokenLimit"`
            OutputTokenLimit   int      `json:"outputTokenLimit"`
            SupportedMethods   []string `json:"supportedGenerationMethods"`
        } `json:"models"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
        return nil, fmt.Errorf("failed to parse Gemini models response: %w", err)
    }

    models := make([]ModelInfo, 0, len(geminiResp.Models))
    for _, m := range geminiResp.Models {
        // Filter to only generation models (exclude embeddings, etc.)
        if !contains(m.SupportedMethods, "generateContent") {
            continue
        }

        models = append(models, ModelInfo{
            ID:              extractModelID(m.Name), // "models/gemini-2.5-flash" -> "gemini-2.5-flash"
            Name:            m.DisplayName,
            Provider:        "gemini",
            Description:     m.Description,
            ContextWindow:   m.InputTokenLimit,
            MaxOutputTokens: m.OutputTokenLimit,
            Capabilities:    inferCapabilities(m),
        })
    }

    return models, nil
}
```

**Similar implementations for:**
- `OpenAIProvider.FetchAvailableModels()`
- `AnthropicProvider.FetchAvailableModels()`

#### 3. Add Caching Layer

**File:** `backend/api/llm/provider/model_cache.go`

```go
package provider

import (
    "context"
    "sync"
    "time"
)

type ModelCache struct {
    mu              sync.RWMutex
    cache           map[ProviderType]cachedModels
    ttl             time.Duration
}

type cachedModels struct {
    models    []ModelInfo
    fetchedAt time.Time
}

func NewModelCache(ttl time.Duration) *ModelCache {
    return &ModelCache{
        cache: make(map[ProviderType]cachedModels),
        ttl:   ttl,
    }
}

func (mc *ModelCache) Get(provider ProviderType) ([]ModelInfo, bool) {
    mc.mu.RLock()
    defer mc.mu.RUnlock()

    cached, exists := mc.cache[provider]
    if !exists {
        return nil, false
    }

    // Check if cache is still valid
    if time.Since(cached.fetchedAt) > mc.ttl {
        return nil, false
    }

    return cached.models, true
}

func (mc *ModelCache) Set(provider ProviderType, models []ModelInfo) {
    mc.mu.Lock()
    defer mc.mu.Unlock()

    mc.cache[provider] = cachedModels{
        models:    models,
        fetchedAt: time.Now(),
    }
}

// Global cache instance with 15-minute TTL
var globalModelCache = NewModelCache(15 * time.Minute)
```

#### 4. Update ProviderFactory

**File:** `backend/api/llm/provider/provider.go`

```go
// GetModelsByProvider returns models for a specific provider
// First checks cache, then fetches from API if needed, falls back to hardcoded list
func (f *ProviderFactory) GetModelsByProvider(providerType ProviderType) ([]ModelInfo, error) {
    provider, err := f.GetProvider(providerType)
    if err != nil {
        return nil, err
    }

    // Try cache first
    if cached, ok := globalModelCache.Get(providerType); ok {
        return cached, nil
    }

    // Try fetching from provider API
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if models, err := provider.FetchAvailableModels(ctx); err == nil {
        globalModelCache.Set(providerType, models)
        return models, nil
    } else {
        // Log the error but continue with fallback
        log.Printf("Failed to fetch models from %s API: %v, using fallback", providerType, err)
    }

    // Fallback to hardcoded list from models.go
    return provider.SupportedModels(), nil
}

// RefreshModels forces a refresh from the provider API
func (f *ProviderFactory) RefreshModels(providerType ProviderType) ([]ModelInfo, error) {
    provider, err := f.GetProvider(providerType)
    if err != nil {
        return nil, err
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    models, err := provider.FetchAvailableModels(ctx)
    if err != nil {
        return nil, err
    }

    globalModelCache.Set(providerType, models)
    return models, nil
}
```

#### 5. Add Refresh Endpoint

**File:** `backend/api/llm/controller.go`

```go
// HandleRefreshModels forces a refresh of models from provider API
func HandleRefreshModels(c echo.Context) error {
    providerName := c.Param("provider")
    if providerName == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider parameter is required"})
    }

    providerType := provider.ProviderType(providerName)
    models, err := providerFactory.RefreshModels(providerType)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{
            "error": fmt.Sprintf("Failed to refresh models: %v", err),
        })
    }

    return c.JSON(http.StatusOK, models)
}
```

**File:** `backend/api/llm/routes.go`

```go
func RegisterRoutes(e *echo.Echo) {
    // ... existing routes ...

    // NEW: Force refresh models from provider API
    e.POST("/llm/providers/:provider/models/refresh", HandleRefreshModels)
}
```

---

### Frontend Changes

#### 1. Add Refresh Button to ModelSelector

**File:** `frontend/src/features/prompt-playground/components/ModelSelector.tsx`

```tsx
import { ReloadIcon } from '@radix-ui/react-icons'

export default function ModelSelector(props: ModelSelectorProps) {
  // ... existing state ...
  const [refreshing, setRefreshing] = useState(false)

  const handleRefresh = async () => {
    if (!provider) return

    setRefreshing(true)
    try {
      const response = await fetch(
        `${API_HOST}/llm/providers/${provider}/models/refresh`,
        {
          method: 'POST',
          credentials: 'include',
        }
      )

      if (response.ok) {
        const refreshedModels = await response.json()
        setModels(refreshedModels)
      }
    } catch (error) {
      console.error('Failed to refresh models:', error)
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <div className="flex items-center gap-2">
      <Select.Root>{/* existing select */}</Select.Root>

      <button
        type="button"
        onClick={handleRefresh}
        disabled={!provider || loading || refreshing}
        className="p-2 rounded hover:bg-gray-100 disabled:opacity-50"
        title="Refresh models from provider API"
      >
        <ReloadIcon className={refreshing ? 'animate-spin' : ''} />
      </button>
    </div>
  )
}
```

#### 2. Display Model Metadata

**File:** `frontend/src/features/prompt-playground/components/ModelSelector.tsx`

```tsx
<Select.Item value={model}>
  <div className="flex flex-col">
    <span className="font-medium">{model.name}</span>
    {model.description && (
      <span className="text-xs text-gray-500">{model.description}</span>
    )}
    {model.contextWindow && (
      <span className="text-xs text-gray-400">
        Context: {model.contextWindow.toLocaleString()} tokens
      </span>
    )}
  </div>
</Select.Item>
```

---

## Testing Plan

### Unit Tests

1. **Backend Provider Tests:**
   - Test `FetchAvailableModels()` for each provider
   - Mock API responses
   - Test error handling (network failure, invalid response)
   - Test fallback to hardcoded lists

2. **Cache Tests:**
   - Test cache hit/miss
   - Test TTL expiration
   - Test concurrent access

### Integration Tests

1. **API Endpoint Tests:**
   - Test `/llm/providers/:provider/models` with caching
   - Test `/llm/providers/:provider/models/refresh`
   - Test error responses

2. **Frontend Tests:**
   - Test refresh button functionality
   - Test model list updates
   - Test loading states

### Manual Testing

1. Test with real API keys for all three providers
2. Verify new models appear without code changes
3. Test cache behavior (15-minute TTL)
4. Test fallback when API calls fail
5. Test refresh button in UI

---

## Deployment Considerations

### Environment Variables

No new environment variables required (uses existing API keys).

### Performance Impact

- **Cache Hit:** No additional latency
- **Cache Miss:** ~1-3 seconds (one-time per provider per 15 minutes)
- **Refresh:** User-initiated, ~1-3 seconds

### Rollback Plan

If dynamic fetching causes issues:
1. Comment out API fetch logic in `GetModelsByProvider()`
2. Returns to hardcoded lists immediately
3. No database changes or migrations needed

---

## Future Enhancements (Beyond Phase 3)

1. **Admin Dashboard:**
   - View cached model lists
   - Manually trigger refreshes
   - Configure cache TTL

2. **Model Recommendations:**
   - Suggest best model based on prompt length
   - Show cost estimates
   - Highlight new models

3. **Model Comparisons:**
   - Side-by-side comparison of model outputs
   - Performance benchmarks
   - Cost analysis

4. **Custom Model Support:**
   - Add user's fine-tuned models
   - Azure OpenAI deployments
   - AWS Bedrock custom models

---

## Files to Create/Modify

### Backend (Go)

**New Files:**
- `backend/api/llm/provider/model_cache.go` - Caching implementation

**Modified Files:**
- `backend/api/llm/provider/provider.go` - Update interface, add caching methods
- `backend/api/llm/provider/gemini_provider.go` - Add `FetchAvailableModels()`
- `backend/api/llm/provider/openai_provider.go` - Add `FetchAvailableModels()`
- `backend/api/llm/provider/anthropic_provider.go` - Add `FetchAvailableModels()`
- `backend/api/llm/controller.go` - Add `HandleRefreshModels()`
- `backend/api/llm/routes.go` - Add refresh endpoint
- `backend/api/llm/schemas.go` - Enhance `ModelInfo` struct

### Frontend (TypeScript/React)

**Modified Files:**
- `frontend/src/features/prompt-playground/components/ModelSelector.tsx` - Add refresh button, show metadata
- `frontend/src/features/prompt-playground/schemas/unified-request.ts` - Update `ModelInfo` type
- `frontend/src/features/prompt-playground/fetch/unified-llm-request.ts` - Add refresh function

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Provider API changes format | High | Fallback to hardcoded lists |
| Provider API rate limiting | Medium | 15-minute cache, manual refresh only |
| Slow API responses | Low | 10-second timeout, caching |
| API key permissions | Medium | Graceful error handling, fallback |
| Network failures | Low | Fallback to hardcoded lists |

---

## Success Metrics

- ✅ New models appear automatically within 15 minutes
- ✅ Zero code changes required to support new models
- ✅ <1% increase in API response time (due to caching)
- ✅ 100% uptime (fallback to hardcoded lists on failures)
- ✅ Positive user feedback on refresh feature

---

## References

- [OpenAI Models API Documentation](https://platform.openai.com/docs/api-reference/models/list)
- [Anthropic Models API Documentation](https://docs.anthropic.com/en/api/models-list)
- [Google Gemini Models API Documentation](https://ai.google.dev/api/models)
- [OpenInference Semantic Conventions](https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md)

---

**Created:** 2025-01-XX
**Last Updated:** 2025-01-XX
**Author:** Claude Code
**Status:** Planned (Not Implemented)

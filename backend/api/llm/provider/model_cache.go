package provider

import (
	"sync"
	"time"
)

// ModelCache provides thread-safe caching of model lists with TTL
type ModelCache struct {
	mu    sync.RWMutex
	cache map[ProviderType]cachedModels
	ttl   time.Duration
}

// cachedModels holds the cached model list and fetch timestamp
type cachedModels struct {
	models    []ModelInfo
	fetchedAt time.Time
}

// NewModelCache creates a new model cache with the specified TTL
func NewModelCache(ttl time.Duration) *ModelCache {
	return &ModelCache{
		cache: make(map[ProviderType]cachedModels),
		ttl:   ttl,
	}
}

// Get retrieves models from cache for the given provider
// Returns (models, true) if found and not expired, (nil, false) otherwise
func (mc *ModelCache) Get(provider ProviderType) ([]ModelInfo, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	cached, exists := mc.cache[provider]
	if !exists {
		return nil, false
	}

	// Check if cache entry has expired
	if time.Since(cached.fetchedAt) > mc.ttl {
		return nil, false
	}

	return cached.models, true
}

// Set stores models in cache for the given provider
func (mc *ModelCache) Set(provider ProviderType, models []ModelInfo) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.cache[provider] = cachedModels{
		models:    models,
		fetchedAt: time.Now(),
	}
}

// Clear removes all entries from the cache
func (mc *ModelCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.cache = make(map[ProviderType]cachedModels)
}

// ClearProvider removes the cache entry for a specific provider
func (mc *ModelCache) ClearProvider(provider ProviderType) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.cache, provider)
}

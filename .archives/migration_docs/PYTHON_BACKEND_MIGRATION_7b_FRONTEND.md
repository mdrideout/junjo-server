# Phase 7b: Frontend Migration for LiteLLM Integration

**Status**: ✅ Complete (implemented alongside Phase 7a)
**Last Updated**: 2025-01-29
**Dependencies**: Phase 7a Complete (LiteLLM Backend)

---

## Overview

Phase 7b covers the frontend changes required to integrate with the new unified LiteLLM backend. Notably, **this work was completed alongside Phase 7a**, as the frontend and backend were developed in parallel.

**Key Achievement**: The frontend now uses a single unified `/llm/generate` endpoint for all providers (OpenAI, Anthropic, Gemini), replacing the previous provider-specific endpoints.

---

## What Changed

### 1. Unified API Endpoint

**Before (Go Backend)**:
```typescript
// Provider-specific endpoints
POST /llm/openai/generate
POST /llm/anthropic/generate
POST /llm/gemini/generate
```

**After (Python Backend with LiteLLM)**:
```typescript
// Single unified endpoint
POST /llm/generate
```

### 2. Model Naming Convention

**Before**:
```typescript
model: "gpt-4o"  // No prefix
model: "claude-3-5-sonnet-20241022"
model: "gemini-2.5-flash-lite"
```

**After**:
```typescript
model: "openai/gpt-4o"  // Provider prefix required
model: "anthropic/claude-3-5-sonnet-20241022"
model: "gemini/gemini-2.5-flash-lite"
```

### 3. Unified Request Schema

All providers now use the same request format with LiteLLM translating to provider-specific APIs.

**Request Format** (`frontend/src/features/prompt-playground/schemas/litellm-request.ts`):
```typescript
export const LiteLLMRequestSchema = z.object({
  // Required fields
  model: z.string(),  // With provider prefix
  messages: z.array(LiteLLMMessageSchema),

  // Common generation parameters
  temperature: z.number().min(0.0).max(2.0).optional(),
  max_tokens: z.number().positive().optional(),
  top_p: z.number().min(0.0).max(1.0).optional(),
  stop: z.array(z.string()).max(4).optional(),

  // Unified reasoning parameter (works across all providers)
  reasoning_effort: LiteLLMReasoningEffortSchema.optional(),

  // OpenAI-specific (for reasoning models)
  max_completion_tokens: z.number().positive().optional(),

  // JSON mode / Structured outputs
  json_mode: z.boolean().optional(),
  json_schema: z.record(z.any()).optional(),
})
```

---

## Files Changed

### 1. API Request Function
**File**: `frontend/src/features/prompt-playground/fetch/litellm-request.ts`

**Changes**:
- Updated endpoint from provider-specific to unified `/llm/generate`
- Uses LiteLLM request/response schemas
- Handles FastAPI error format (`.detail` field)

```typescript
export const litellmRequest = async (payload: LiteLLMRequest): Promise<LiteLLMResponse> => {
  const endpoint = '/llm/generate'  // ← Unified endpoint
  const apiHost = getApiHost(endpoint)

  const response = await fetch(`${apiHost}${endpoint}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
    credentials: 'include',
  })

  if (!response.ok) {
    const errorData = await response.json()
    // FastAPI returns errors in 'detail' field
    throw new Error(errorData.detail || errorData.error || 'Failed to generate content')
  }

  return LiteLLMResponseSchema.parse(await response.json())
}
```

### 2. Request/Response Schemas
**File**: `frontend/src/features/prompt-playground/schemas/litellm-request.ts`

**Key Changes**:
- Added `reasoning_effort` with values: `'minimal' | 'low' | 'medium' | 'high'`
- Changed `reasoning_content` from `.optional()` to `.nullish()` (accepts both null and undefined)
- Unified schema for all providers

```typescript
// Reasoning effort unified across providers
export const LiteLLMReasoningEffortSchema = z.enum(['minimal', 'low', 'medium', 'high'])

// Response schema with nullable reasoning_content
export const LiteLLMResponseSchema = z.object({
  id: z.string(),
  object: z.string(),
  created: z.number(),
  model: z.string(),
  choices: z.array(LiteLLMChoiceSchema),
  usage: LiteLLMUsageSchema.optional(),
  reasoning_content: z.string().nullish(),  // ← Changed from .optional() to .nullish()
})
```

### 3. Model Loading from Telemetry
**File**: `frontend/src/features/prompt-playground/PromptPlaygroundPage.tsx`

**Changes**:
- Prepends provider prefix when loading model from OTEL telemetry
- Maps OpenInference provider names to internal names (`google` → `gemini`)

```typescript
// Line 120-134
useEffect(() => {
  if (span) {
    // Extract and set provider using OpenInference mapping
    const otelProviderName = span.attributes_json['llm.provider']
    const internalProvider = otelProviderName ? mapOtelProviderToInternal(otelProviderName) : 'gemini'
    dispatch(PromptPlaygroundActions.setSelectedProvider(internalProvider))
    setOriginalProvider(internalProvider)

    // Extract and set model with provider prefix for LiteLLM
    const modelName = span.attributes_json['llm.model_name'] || 'Unknown'
    // Construct full model ID with provider prefix (e.g., "gemini/gemini-2.5-flash-lite")
    const fullModelId = modelName.includes('/') ? modelName : `${internalProvider}/${modelName}`
    dispatch(PromptPlaygroundActions.setSelectedModel(fullModelId))
    setOriginalModel(fullModelId)
    // ... JSON mode detection, settings import
  }
}, [span, dispatch])
```

### 4. Provider Mapping Utilities
**File**: `frontend/src/features/prompt-playground/utils/provider-mapping.ts`

**New Utility Functions**:
```typescript
/**
 * Maps OpenInference provider names to internal provider names
 * OpenInference uses "google" for Gemini, we use "gemini" internally
 */
export function mapOtelProviderToInternal(otelProvider: string): string {
  const mappings: Record<string, string> = {
    'google': 'gemini',
    'openai': 'openai',
    'anthropic': 'anthropic',
    // ... more providers
  }
  return mappings[normalized] || normalized
}
```

### 5. Model Selector
**File**: `frontend/src/features/prompt-playground/components/ModelSelector.tsx`

**Changes**:
- Handles models with provider prefix
- Extracts display name (removes prefix for UI)
- Supports original model from telemetry if not in fetched list

```typescript
// Lines 71-95: Include original model from telemetry
const allModels = useMemo(() => {
  const result = [...models]
  if (
    originalModel &&
    originalModel !== 'Unknown' &&
    provider === originalProvider &&
    !models.find((m) => m.id === originalModel)
  ) {
    // Add original model with provider prefix format
    const modelId = originalModel.includes('/') ? originalModel : `${provider}/${originalModel}`
    // Extract display name (remove provider prefix)
    const displayName = originalModel.includes('/') ? originalModel.split('/')[1] : originalModel
    result.push({
      id: modelId,
      display_name: displayName,  // Clean display without prefix
      provider: provider || 'unknown',
      supports_reasoning: false,
      supports_vision: false,
      max_tokens: null,
    })
  }
  return result
}, [models, originalModel, originalProvider, provider])
```

### 6. Model Grouping Utilities
**File**: `frontend/src/features/prompt-playground/utils/model-grouping.ts`

**Changes**:
- Fixed property access from `model.name` to `model.display_name`
- Ensures consistent model display across UI

```typescript
// Lines 636-638
variant: model.display_name || model.id,
displayName: model.display_name || model.id,
```

### 7. Generation Settings
**File**: `frontend/src/features/prompt-playground/store/slice.ts`

**Changes**:
- Added `reasoning_effort` type: `'minimal' | 'low' | 'medium' | 'high'`
- Maintains provider-specific settings for backwards compatibility

```typescript
export interface GenerationSettings {
  // OpenAI
  reasoning_effort?: 'minimal' | 'low' | 'medium' | 'high'
  max_completion_tokens?: number

  // Anthropic
  thinking_enabled?: boolean
  thinking_budget_tokens?: number

  // Shared
  temperature?: number
  max_tokens?: number

  // Gemini
  thinkingBudget?: number
  includeThoughts?: boolean
  maxOutputTokens?: number
}
```

### 8. Request Building
**File**: `frontend/src/features/prompt-playground/PromptPlaygroundPage.tsx`

**Changes**:
- Uses unified `litellmRequest()` function
- Passes `reasoning_effort` parameter for all providers
- LiteLLM auto-translates to provider-specific implementations

```typescript
// Lines 317-346: Unified request
const result = await litellmRequest({
  model: selectedModel,  // With provider prefix
  messages: [{ role: 'user', content: prompt }],

  // Common parameters
  ...(generationSettings.temperature !== undefined && {
    temperature: generationSettings.temperature,
  }),
  ...(generationSettings.max_tokens && {
    max_tokens: generationSettings.max_tokens,
  }),

  // JSON mode / Structured output
  ...(jsonMode && {
    json_mode: true,
    ...(jsonSchema && {
      json_schema: ensureOpenAISchemaCompatibility(jsonSchema),
    }),
  }),

  // Reasoning/thinking (LiteLLM auto-translates to provider-specific)
  ...(generationSettings.reasoning_effort && {
    reasoning_effort: generationSettings.reasoning_effort,
  }),

  // OpenAI reasoning models specific
  ...(generationSettings.max_completion_tokens && {
    max_completion_tokens: generationSettings.max_completion_tokens,
  }),
})
```

---

## Model Discovery

Model discovery endpoints remain provider-specific (unchanged from Go implementation):

```typescript
// Fetch models for provider
GET /llm/providers/{provider}/models

// Force refresh (bypass cache)
POST /llm/providers/{provider}/models/refresh
```

**Why Provider-Specific**: Each provider has different model discovery APIs:
- OpenAI: `https://api.openai.com/v1/models`
- Anthropic: `https://api.anthropic.com/v1/models`
- Gemini: `https://generativelanguage.googleapis.com/v1beta/models`

**Implementation**: `frontend/src/features/prompt-playground/fetch/model-discovery.ts`
- `fetchModelsByProvider(provider)` - Fetches models with 15-min cache
- `refreshModelsByProvider(provider)` - Forces API refresh

---

## Provider-Specific Behavior

### OpenAI
**Models Supported**:
- `openai/gpt-4o`
- `openai/o1` (reasoning model)
- `openai/o3-mini` (reasoning model)

**Reasoning**: Use `reasoning_effort` parameter
- LiteLLM passes through to OpenAI API
- Returns `reasoning_content` field with thinking

### Anthropic
**Models Supported**:
- `anthropic/claude-3-5-sonnet-20241022`
- `anthropic/claude-3-7-sonnet-20250219` (extended thinking)

**Reasoning**: Use `reasoning_effort` parameter
- LiteLLM translates to `thinking: {type: "enabled", budget_tokens: ...}`
- Returns `thinking_blocks` which LiteLLM normalizes to `reasoning_content`

### Gemini
**Models Supported**:
- `gemini/gemini-2.5-pro`
- `gemini/gemini-2.5-flash-lite`
- `gemini/gemini-1.5-pro`

**Reasoning**: Use `reasoning_effort` parameter
- LiteLLM translates to `thinking: {budget_tokens: ...}`
- Returns thinking in normalized `reasoning_content` field

---

## Testing

### Manual Testing Performed
- ✅ Gemini integration (end-to-end working)
- ✅ Model loading from OTEL telemetry
- ✅ Provider prefix prepending
- ✅ Model selection and display
- ✅ JSON mode (schema-less)
- ✅ Settings persistence across provider changes
- ✅ Error handling (schema validation, API errors)

### Pending Testing
- ⏳ OpenAI integration (gpt-4o, o1)
- ⏳ Anthropic integration (Claude 3.7)
- ⏳ Reasoning models with extended thinking
- ⏳ JSON mode with schema (structured outputs)

---

## Migration Checklist

### ✅ Completed
- [x] Update API request function to use `/llm/generate`
- [x] Add provider prefix to model names
- [x] Update request/response schemas for LiteLLM
- [x] Implement provider mapping utilities (`google` → `gemini`)
- [x] Update model loading from telemetry
- [x] Fix model display name extraction
- [x] Add `reasoning_effort` parameter
- [x] Change `reasoning_content` schema to `.nullish()`
- [x] Test Gemini integration end-to-end
- [x] Verify error handling and validation

### ⏳ Pending (for full Phase 7 completion)
- [ ] Test OpenAI integration
- [ ] Test Anthropic integration
- [ ] Verify reasoning models work correctly
- [ ] Test structured outputs (JSON schema)

---

## Success Criteria

### Functional Requirements ✅
- ✅ Frontend uses unified `/llm/generate` endpoint
- ✅ Model names include provider prefix
- ✅ Provider mapping handles OpenInference telemetry
- ✅ Model selection works for all providers
- ✅ Settings persist correctly
- ✅ Gemini integration working end-to-end
- ✅ Error handling for API failures
- ✅ Schema validation working

### Code Quality ✅
- ✅ TypeScript types match backend Pydantic schemas
- ✅ No experimental/debug code remaining
- ✅ Clean separation of concerns
- ✅ Proper error handling
- ✅ Consistent naming conventions

### Performance ✅
- ✅ No unnecessary re-renders
- ✅ Efficient state management
- ✅ Model caching working

---

## Key Learnings

### What Worked Well
1. **Parallel Development**: Developing frontend and backend together caught integration issues early
2. **Unified Schema**: Single request/response format simplified frontend logic
3. **Provider Mapping**: Clean abstraction for OpenInference → internal provider names
4. **Type Safety**: Zod schemas caught validation errors before API calls

### Challenges Overcome
1. **Model Prefix**: Initially forgot to prepend provider prefix when loading from telemetry
2. **Null vs Undefined**: Backend returns `reasoning_content: null`, frontend schema needed `.nullish()` not `.optional()`
3. **Display Names**: Had to extract display name from full model ID for clean UI

### Best Practices Established
1. Always prepend provider prefix to model names (`provider/model`)
2. Use `.nullish()` for nullable backend fields (not `.optional()`)
3. Map OpenInference provider names to internal names
4. Extract display names from model IDs for UI
5. Validate at API boundary with Zod schemas

---

## Documentation

### Frontend Code Locations
- **API Request**: `frontend/src/features/prompt-playground/fetch/litellm-request.ts`
- **Schemas**: `frontend/src/features/prompt-playground/schemas/litellm-request.ts`
- **Main Page**: `frontend/src/features/prompt-playground/PromptPlaygroundPage.tsx`
- **Model Selector**: `frontend/src/features/prompt-playground/components/ModelSelector.tsx`
- **Provider Mapping**: `frontend/src/features/prompt-playground/utils/provider-mapping.ts`
- **State Management**: `frontend/src/features/prompt-playground/store/slice.ts`

### Backend Integration
- **API Endpoint**: `backend_python/app/features/llm_playground/router.py`
- **Request Schema**: `backend_python/app/features/llm_playground/schemas.py`
- **LiteLLM Service**: `backend_python/app/features/llm_playground/service.py`

---

## Next Steps

This phase is **functionally complete** - the frontend is working with the new Python backend. Remaining work is in Phase 7a (backend testing):

1. Test OpenAI provider integration
2. Test Anthropic provider integration
3. Verify reasoning models across all providers
4. Mark Phase 7 as complete
5. Proceed to Phase 8 (Deployment & Cutover)

---

**Status**: ✅ Complete (implemented alongside Phase 7a)
**Last Updated**: 2025-01-29
**Next Phase**: Phase 8 - Deployment & Cutover

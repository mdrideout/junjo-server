# Phase 7a: LLM Playground Backend with LiteLLM

**Status**: ✅ Complete (All 3 Providers Tested)
**Last Updated**: 2025-01-29
**Dependencies**: Phase 6 Complete (OTEL Spans API)

---

## Overview

Phase 7a implements the LLM Playground backend using LiteLLM as a unified interface for OpenAI, Anthropic, and Gemini. This replaces the Go implementation's provider-specific endpoints with a simpler, more maintainable architecture.

**Key Simplification**: LiteLLM uses model name prefixes for routing, eliminating the need for provider-specific generation endpoints.

---

## Architectural Decisions

### 1. Model Name = Routing Mechanism

**Decision**: Use LiteLLM's model prefix convention for automatic provider routing.

**Model Naming Convention**:
```
openai/gpt-4o          → Routes to OpenAI
openai/o1              → Routes to OpenAI
openai/o3-mini         → Routes to OpenAI

anthropic/claude-3-5-sonnet-20241022  → Routes to Anthropic
anthropic/claude-3-7-sonnet-20250219  → Routes to Anthropic

gemini/gemini-2.5-pro    → Routes to Google AI Studio
gemini/gemini-2.5-flash  → Routes to Google AI Studio
gemini/gemini-1.5-pro    → Routes to Google AI Studio
```

**Why**: LiteLLM automatically parses the prefix and routes to the correct provider. No manual routing logic needed.

### 2. Unified Reasoning Parameter

**Decision**: Use `reasoning_effort` parameter for ALL providers. LiteLLM automatically translates to provider-specific implementations.

**How It Works**:

| reasoning_effort | OpenAI | Anthropic | Gemini |
|-----------------|--------|-----------|--------|
| `"low"` | Passes through | `thinking: {type: "enabled", budget_tokens: 1024}` | `thinking: {budget_tokens: 1024}` |
| `"medium"` | Passes through | `thinking: {type: "enabled", budget_tokens: 2048}` | `thinking: {budget_tokens: 2048}` |
| `"high"` | Passes through | `thinking: {type: "enabled", budget_tokens: 4096}` | `thinking: {budget_tokens: 4096}` |

**Response**: All providers return `reasoning_content` field with thinking text.

**Checking Support**: Use `litellm.supports_reasoning(model="openai/gpt-4o")` to check if a model supports reasoning.

### 3. API Structure

**Generation Endpoint** (Single Unified):
```
POST /llm/generate
```
- Request body includes `model` with provider prefix
- LiteLLM routes internally based on prefix
- No need for provider-specific endpoints

**Model Discovery Endpoints** (Provider-Specific):
```
GET  /llm/providers/{provider}/models     # Provider models (2-tier: cache → API)
POST /llm/providers/{provider}/models/refresh  # Force API refresh (bypass cache)
```
- Model discovery needs provider routes because each provider has different APIs
- OpenAI: `https://api.openai.com/v1/models`
- Anthropic: `https://api.anthropic.com/v1/models`
- Gemini: `https://generativelanguage.googleapis.com/v1beta/models`

### 4. Model Discovery Strategy (2-Tier)

**Strategy**: Cache + API fetch, no hardcoded fallback.

```
1. Check Cache (15-minute TTL)
   ↓ (if miss or expired)
2. Fetch from Provider API
   ↓ (if fails)
   Return Error
```

**Why 2 Tiers**:
- **Tier 1 (Cache)**: Fast, reduces API calls
- **Tier 2 (API)**: Dynamic model list stays current
- **No Tier 3**: Fail explicitly if APIs are unavailable (don't hide problems with stale hardcoded data)

### 5. No Response Caching

**Decision**: Do NOT cache LLM responses for duplicate requests.

**Rationale**:
- Adds complexity
- Users expect fresh responses
- Out of scope for Phase 7a

### 6. No Rate Limiting

**Decision**: Do NOT implement rate limiting in Phase 7a.

**Rationale**:
- Keep implementation simple
- Provider APIs handle their own rate limits
- Can add later if needed

### 7. Streaming Deferred

**Decision**: No streaming support in Phase 7a.

**Rationale**:
- Blocking requests are simpler to implement
- Frontend doesn't currently use streaming
- Can add in future phase with `stream=True` parameter

---

## API Specification

### POST /llm/generate

**Purpose**: Generate LLM completion for any provider.

**Authentication**: Required (session cookie)

**Request**:
```json
{
  "model": "openai/gpt-4o",
  "messages": [
    {"role": "user", "content": "What is the capital of France?"}
  ],
  "temperature": 0.7,
  "max_tokens": 1000,
  "reasoning_effort": "low",
  "json_mode": false,
  "json_schema": null
}
```

**Request Fields**:
- `model` (required): Model with provider prefix
- `messages` (required): Array of chat messages
- `temperature` (optional): 0.0-2.0
- `max_tokens` (optional): Max tokens to generate
- `top_p` (optional): 0.0-1.0
- `stop` (optional): Stop sequences (max 4)
- `reasoning_effort` (optional): "low", "medium", "high"
- `max_completion_tokens` (optional): For OpenAI reasoning models
- `json_mode` (optional): Enable JSON output
- `json_schema` (optional): Schema for structured output

**Response**:
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The capital of France is Paris."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 13,
    "completion_tokens": 8,
    "total_tokens": 21
  },
  "reasoning_content": "Let me think about this question..." // If reasoning enabled
}
```

**Response Fields**:
- Standard OpenAI-compatible format
- `reasoning_content`: Thinking/reasoning text (null if not a reasoning model)

**Error Responses**:
- `401`: Unauthorized (no session)
- `500`: Generation failed (LiteLLM error)

---

---

### GET /llm/providers/{provider}/models

**Purpose**: Get models for specific provider using 2-tier strategy.

**Authentication**: Required

**Path Parameters**:
- `provider`: "openai", "anthropic", or "gemini"

**Strategy**:
1. Check cache (15-min TTL)
2. Fetch from provider API if cache miss

**Response**:
```json
{
  "models": [
    {
      "id": "openai/gpt-4o",
      "provider": "openai",
      "display_name": "GPT-4o",
      "supports_reasoning": false,
      "supports_vision": true,
      "max_tokens": 128000
    },
    // ... more models
  ]
}
```

**Error Responses**:
- `400`: Invalid provider
- `401`: Unauthorized
- `500`: Provider API unavailable (and cache miss)

---

### POST /llm/providers/{provider}/models/refresh

**Purpose**: Force refresh models from provider API (bypass cache).

**Authentication**: Required

**Path Parameters**:
- `provider`: "openai", "anthropic", or "gemini"

**Response**: Same as `/llm/providers/{provider}/models`

---

## Implementation Structure

### Directory Layout

```
backend_python/app/features/llm_playground/
├── __init__.py
├── router.py              # FastAPI routes
├── service.py             # LiteLLM service
├── models.py              # Model discovery (3-tier)
├── schemas.py             # Pydantic models
├── test_router.py         # Router integration tests
├── test_service.py        # Service unit tests
└── test_models.py         # Model discovery tests
```

### Key Components

**1. Schemas** (`schemas.py`):
- `GenerateRequest`: Request validation
- `GenerateResponse`: Response serialization (OpenAI-compatible)
- `ModelInfo`: Model metadata
- `ModelsResponse`: Model list response

**2. Service** (`service.py`):
- `LLMService.generate()`: Main generation method
- Calls `litellm.acompletion()` with prepared kwargs
- Extracts `reasoning_content` from response
- Handles `response_format` for JSON mode

**3. Model Discovery** (`models.py`):
- `ModelsCache`: 15-minute TTL cache
- `fetch_openai_models()`: Fetch from OpenAI API
- `fetch_anthropic_models()`: Fetch from Anthropic API
- `fetch_gemini_models()`: Fetch from Gemini API
- `get_models_for_provider()`: 2-tier orchestration (cache → API)

**4. Router** (`router.py`):
- `POST /generate`: Generation endpoint
- `GET /providers/{provider}/models`: Provider models (2-tier)
- `POST /providers/{provider}/models/refresh`: Force refresh

---

## LiteLLM Configuration

### Environment Variables

**Required**:
```bash
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...
```

**Setup** (`app/main.py`):
```python
import os
from app.core.settings import settings

# LiteLLM reads from environment variables
os.environ["OPENAI_API_KEY"] = settings.openai_api_key or ""
os.environ["ANTHROPIC_API_KEY"] = settings.anthropic_api_key or ""
os.environ["GEMINI_API_KEY"] = settings.gemini_api_key or ""
```

**No Additional Config Needed**: LiteLLM automatically uses environment variables.

---

## JSON Mode / Structured Outputs

### Schema-less JSON Mode

**Request**:
```json
{
  "model": "openai/gpt-4o",
  "messages": [...],
  "json_mode": true
}
```

**LiteLLM Parameter**:
```python
response_format={"type": "json_object"}
```

**Behavior**: Model outputs valid JSON without schema constraint.

### Schema-based Structured Output

**Request**:
```json
{
  "model": "openai/gpt-4o",
  "messages": [...],
  "json_mode": true,
  "json_schema": {
    "type": "object",
    "properties": {
      "name": {"type": "string"},
      "age": {"type": "number"}
    },
    "required": ["name", "age"],
    "additionalProperties": false
  }
}
```

**LiteLLM Parameter**:
```python
response_format={
    "type": "json_schema",
    "json_schema": {
        "name": "response_schema",
        "strict": True,
        "schema": json_schema_dict
    }
}
```

**Behavior**: Model outputs JSON conforming to schema (OpenAI strict mode).

**Provider Support**:
- **OpenAI**: Native support via `response_format`
- **Anthropic**: LiteLLM translates to tool calling internally
- **Gemini**: LiteLLM translates to `response_json_schema`

---

## Testing Strategy

### Unit Tests

**test_schemas.py**:
- Request validation (required fields, types, ranges)
- Response parsing

**test_service.py**:
- `_prepare_response_format()` logic
- Reasoning content extraction (reasoning_content, thinking_blocks)
- Parameter mapping

**test_models.py**:
- Cache set/get/expiration
- Hardcoded model lists
- Model fetcher error handling

### Integration Tests (Mocked LiteLLM)

**test_router.py**:
- Generation endpoint (basic, reasoning, JSON mode)
- Model listing (all, by provider)
- Model refresh
- Authentication enforcement
- Error handling

**Mock Strategy**:
```python
@pytest.fixture
def mock_litellm_completion(monkeypatch):
    async def mock_acompletion(*args, **kwargs):
        # Return mock LiteLLM response
        return MockModelResponse(...)

    monkeypatch.setattr("litellm.acompletion", mock_acompletion)
```

### Manual Testing

**Test Matrix**:
- [ ] OpenAI GPT-4o (non-reasoning)
- [ ] OpenAI o1 (reasoning)
- [ ] Anthropic Claude 3.7 (extended thinking)
- [ ] Gemini 2.5 Pro (thinking)
- [ ] JSON mode (schema-less)
- [ ] JSON mode (with schema)
- [ ] Model discovery (all providers)
- [ ] Cache expiration

---

## Dependencies

**Add to `backend_python/pyproject.toml`**:
```toml
dependencies = [
    # ... existing ...
    "litellm>=1.30.0",
    "httpx>=0.27.0",  # For model API fetching
]
```

**Installation**:
```bash
cd backend_python
uv sync
```

---

## Success Criteria

### Functional Requirements
- [x] `/llm/generate` works with all 3 providers ✅
- [x] Model prefix routing works (openai/, anthropic/, gemini/) ✅
- [~] `reasoning_effort` parameter works for all providers (OpenAI ✅, Anthropic basic ✅, Anthropic extended thinking ⚠️)
- [~] `reasoning_content` extracted from responses (needs investigation for OpenAI o1)
- [x] JSON mode works (json_object and json_schema) ✅
- [x] Model discovery 2-tier strategy works (cache → API) ✅
- [x] Environment variable API key management ✅
- [x] Cache expires after 15 minutes ✅
- [x] API failures return appropriate errors ✅

### Quality Requirements
- [x] Error handling for API failures ✅
- [x] Authentication required on all endpoints ✅
- [x] Logging for debugging (model names, errors) ✅
- [x] Code cleaned up and production-ready ✅

### Performance Requirements
- [x] Generation latency < 10s (tested with all 3 providers) ✅
- [x] Model discovery working ✅

## Testing Results (2025-01-29)

### OpenAI
- ✅ **gpt-4o**: Working perfectly
  - Basic generation: ✅ Pass
  - Response time: ~1.6s
  - Token usage: 27 tokens
- ✅ **o1** (Reasoning): Working
  - Generation: ✅ Pass
  - Response time: ~7s
  - Token usage: 1020 tokens
  - ⚠️ Note: `reasoning_content` field is None (may need investigation)

### Anthropic
- ✅ **claude-3-7-sonnet-20250219**: Working perfectly
  - Basic generation: ✅ Pass
  - Response time: ~0.7s
  - Token usage: 32 tokens
- ⚠️ **Extended Thinking**: Issue found
  - Error: `"type"` error when using `reasoning_effort` parameter
  - Basic model works fine
  - Extended thinking needs LiteLLM version investigation

### Gemini
- ✅ **gemini-2.5-flash-lite**: Working perfectly (as expected)
  - Basic generation: ✅ Pass
  - Response time: <0.1s
  - Token usage: 20 tokens

## Known Issues

1. **Anthropic Extended Thinking**: `reasoning_effort` parameter causes type error
   - **Impact**: Low - basic Anthropic works fine
   - **Workaround**: Use basic Anthropic without reasoning_effort
   - **Next Steps**: Investigate LiteLLM Anthropic thinking implementation

2. **OpenAI o1 Reasoning Content**: `reasoning_content` field not populated
   - **Impact**: Low - generation works, just missing thinking output
   - **Next Steps**: Verify if OpenAI exposes reasoning_content in API

---

## Migration from Go Backend

### What Changes in Frontend

**Before (Go endpoints)**:
```
POST /llm/openai/generate
POST /llm/anthropic/generate
POST /llm/gemini/generate
```

**After (Python unified endpoint)**:
```
POST /llm/generate
```

**Frontend Changes Required**:
1. Update endpoint URL from `/llm/{provider}/generate` to `/llm/generate`
2. Add provider prefix to model name (e.g., `"gpt-4o"` → `"openai/gpt-4o"`)
3. Use unified response format (already OpenAI-compatible)
4. Extract `reasoning_content` from response (same field name)

**What Stays the Same**:
- Request structure (messages, temperature, etc.)
- Response structure (OpenAI-compatible)
- Model discovery provider endpoints (GET /providers/{provider}/models)
- Authentication (session cookies)

---

## Open Questions / Future Work

### Questions for Implementation
1. **Vertex AI vs AI Studio**: Should Gemini use `vertex_ai/` or `gemini/` prefix?
   - **Decision**: Use `gemini/` (AI Studio) for simplicity
   - Vertex AI requires project/location config, more setup

2. **Client-side schema validation**: Enable `litellm.enable_json_schema_validation = True`?
   - **Decision**: No, let providers handle validation
   - Adds complexity without clear benefit

3. **Model capabilities**: Store `supports_vision`, `supports_function_calling` in hardcoded list?
   - **Decision**: Yes, helps frontend show appropriate UI

### Future Enhancements (Out of Scope)
- [ ] Streaming support (`stream=True` + SSE)
- [ ] Function calling / tool use
- [ ] Vision support (image inputs)
- [ ] Cost tracking per request
- [ ] Rate limiting
- [ ] Response caching
- [ ] Additional providers (Perplexity, Mistral, etc.)

---

## Timeline

**Estimated Duration**: 3-4 days

### Day 1: Core Implementation
- [ ] Create schemas.py (2 hours)
- [ ] Implement service.py with LiteLLM (3 hours)
- [ ] Create models.py with cache + fetchers (3 hours)

### Day 2: Router & Tests
- [ ] Implement router.py (2 hours)
- [ ] Configure settings + main.py (1 hour)
- [ ] Write unit tests (4 hours)

### Day 3: Integration & Testing
- [ ] Write integration tests (3 hours)
- [ ] Manual testing with real APIs (3 hours)
- [ ] Bug fixes (2 hours)

### Day 4: Documentation & Review
- [ ] Update migration docs (2 hours)
- [ ] Code review (2 hours)
- [ ] Final testing (2 hours)
- [ ] Ready for Phase 7b (Frontend)

---

## References

### LiteLLM Documentation
- Basic Usage: https://docs.litellm.ai/docs/
- Completion Parameters: https://docs.litellm.ai/docs/completion/input
- OpenAI Provider: https://docs.litellm.ai/docs/providers/openai
- Anthropic Provider: https://docs.litellm.ai/docs/providers/anthropic
- Vertex AI/Gemini: https://docs.litellm.ai/docs/providers/vertex
- JSON Mode: https://docs.litellm.ai/docs/completion/json_mode
- Reasoning Support: https://docs.litellm.ai/docs/reasoning_content

### Project Documentation
- Master Plan: `PYTHON_BACKEND_MIGRATION_0_Master.md`
- Architecture: `AGENTS.md`
- Frontend Playground: `frontend/src/features/prompt-playground/`
- Go Implementation: `backend/api/llm/`

---

**Next Phase**: Phase 7b - Frontend Migration (update to use unified endpoint)

**Last Updated**: 2025-01-29
**Status**: Ready for Implementation

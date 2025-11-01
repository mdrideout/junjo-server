# Phase 7: LLM Playground with LiteLLM

## Overview

This phase implements the LLM playground feature using LiteLLM for unified access to multiple LLM providers. LiteLLM provides:
- **Unified API**: Single interface for 100+ LLM providers (OpenAI, Anthropic, Gemini, etc.)
- **Automatic OpenTelemetry**: Built-in instrumentation for all LLM calls
- **Streaming Support**: Native streaming for all providers
- **Error Handling**: Standardized error responses across providers
- **Cost Tracking**: Automatic token usage and cost calculation

This replaces the current provider-specific endpoints with a cleaner, more maintainable solution.

## Current Go Implementation Analysis

**Routes** (from `/backend/api/llm/routes.go`):
```go
// Provider-specific endpoints
e.POST("/llm/openai/generate", openai.HandleOpenAIGenerate)
e.POST("/llm/anthropic/generate", anthropic.HandleAnthropicGenerate)
e.POST("/llm/gemini/generate", gemini.HandleGeminiGenerate)

// Model discovery endpoints
e.GET("/llm/models", HandleGetModels)
e.GET("/llm/providers/:provider/models", HandleGetModelsByProvider)
e.POST("/llm/providers/:provider/models/refresh", HandleRefreshModels)
```

**Current Approach**:
- Direct HTTP requests to provider APIs (not using native SDKs)
- Manual request/response mapping for each provider
- No built-in telemetry
- Separate endpoints for each provider
- Model caching with 15-minute TTL
- 3-tier model discovery: cache → API fetch → hardcoded fallback

**API Key Management**:
- Environment variables: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`

**Supported Features** (from OpenAI handler):
- Basic parameters: model, messages, temperature, max_tokens, top_p, stop_sequences
- OpenAI-specific: max_completion_tokens, reasoning_effort
- Response formats: text, json_object, json_schema (structured outputs)

## Python Implementation with LiteLLM

### Directory Structure

```
python_backend/
└── app/
    ├── features/
    │   └── llm_playground/
    │       ├── __init__.py
    │       ├── router.py          # FastAPI endpoints
    │       ├── service.py         # LiteLLM integration
    │       ├── schemas.py         # Request/response schemas
    │       ├── models_cache.py    # Model caching
    │       └── utils.py           # Helper functions
    └── tests/
        ├── unit/
        │   └── features/
        │       └── llm_playground/
        │           ├── test_schemas.py
        │           ├── test_models_cache.py
        │           └── test_service.py
        └── integration/
            └── features/
                └── llm_playground/
                    └── test_llm_endpoints.py
```

### 1. Request/Response Schemas

**File**: `app/features/llm_playground/schemas.py`

```python
"""
Schemas for LLM Playground.

Unified schemas that work across all providers via LiteLLM.
"""

from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field


# --- Chat Completion Schemas ---


class Message(BaseModel):
    """Chat message."""

    role: str = Field(..., description="Role: 'system', 'user', or 'assistant'")
    content: str = Field(..., description="Message content")


class ResponseFormat(BaseModel):
    """Response format configuration."""

    type: str = Field(..., description="Format type: 'text', 'json_object', or 'json_schema'")
    json_schema: Optional[Dict[str, Any]] = Field(None, description="JSON schema for structured output")


class GenerateRequest(BaseModel):
    """
    Unified LLM generation request.

    Works with all providers via LiteLLM.
    """

    model: str = Field(..., description="Model name (e.g., 'gpt-4', 'claude-3-opus', 'gemini-pro')")
    messages: List[Message] = Field(..., description="List of chat messages")

    # Common parameters (work across all providers)
    temperature: Optional[float] = Field(None, ge=0.0, le=2.0, description="Sampling temperature")
    max_tokens: Optional[int] = Field(None, gt=0, description="Maximum tokens to generate")
    top_p: Optional[float] = Field(None, ge=0.0, le=1.0, description="Nucleus sampling parameter")
    stop: Optional[List[str]] = Field(None, description="Stop sequences")

    # OpenAI-specific parameters
    max_completion_tokens: Optional[int] = Field(None, gt=0, description="Max completion tokens (OpenAI o1 models)")
    reasoning_effort: Optional[str] = Field(None, description="Reasoning effort (OpenAI o1 models): low, medium, high")

    # Response format
    response_format: Optional[ResponseFormat] = Field(None, description="Response format configuration")

    # Streaming
    stream: bool = Field(False, description="Enable streaming responses")


class Usage(BaseModel):
    """Token usage information."""

    prompt_tokens: int
    completion_tokens: int
    total_tokens: int


class Choice(BaseModel):
    """Generation choice."""

    index: int
    message: Message
    finish_reason: Optional[str] = None


class GenerateResponse(BaseModel):
    """LLM generation response."""

    id: str
    object: str
    created: int
    model: str
    choices: List[Choice]
    usage: Optional[Usage] = None


# --- Model Discovery Schemas ---


class ModelInfo(BaseModel):
    """Model information."""

    id: str = Field(..., description="Model ID")
    provider: str = Field(..., description="Provider name")
    display_name: str = Field(..., description="Human-readable model name")
    supports_structured_output: bool = Field(False, description="Supports structured output (JSON schema)")
    supports_vision: bool = Field(False, description="Supports vision/image inputs")
    supports_function_calling: bool = Field(False, description="Supports function calling")
    max_tokens: Optional[int] = Field(None, description="Maximum context tokens")
    max_output_tokens: Optional[int] = Field(None, description="Maximum output tokens")


class ModelsResponse(BaseModel):
    """Response for model listing."""

    models: List[ModelInfo]
```

### 2. LLM Service with LiteLLM

**File**: `app/features/llm_playground/service.py`

```python
"""
LLM service using LiteLLM for unified provider access.
"""

import os
from typing import AsyncIterator, Dict, List

import litellm
from litellm import acompletion, completion

from app.core.logger import logger
from app.features.llm_playground.schemas import GenerateRequest, GenerateResponse, Message


class LLMService:
    """Service for LLM operations using LiteLLM."""

    @staticmethod
    def _prepare_litellm_kwargs(request: GenerateRequest) -> Dict:
        """
        Prepare kwargs for LiteLLM completion call.

        Args:
            request: Generate request

        Returns:
            Dict of LiteLLM parameters
        """
        kwargs = {
            "model": request.model,
            "messages": [{"role": msg.role, "content": msg.content} for msg in request.messages],
        }

        # Add optional parameters
        if request.temperature is not None:
            kwargs["temperature"] = request.temperature
        if request.max_tokens is not None:
            kwargs["max_tokens"] = request.max_tokens
        if request.top_p is not None:
            kwargs["top_p"] = request.top_p
        if request.stop:
            kwargs["stop"] = request.stop

        # OpenAI-specific parameters
        if request.max_completion_tokens is not None:
            kwargs["max_completion_tokens"] = request.max_completion_tokens
        if request.reasoning_effort is not None:
            kwargs["reasoning_effort"] = request.reasoning_effort

        # Response format
        if request.response_format:
            if request.response_format.type == "json_schema" and request.response_format.json_schema:
                kwargs["response_format"] = {
                    "type": "json_schema",
                    "json_schema": request.response_format.json_schema,
                }
            elif request.response_format.type == "json_object":
                kwargs["response_format"] = {"type": "json_object"}

        # Streaming
        kwargs["stream"] = request.stream

        return kwargs

    @staticmethod
    async def generate(request: GenerateRequest) -> GenerateResponse:
        """
        Generate LLM completion.

        Uses LiteLLM for unified access to all providers.

        Args:
            request: Generate request

        Returns:
            Generate response

        Raises:
            Exception: If generation fails
        """
        try:
            # Prepare LiteLLM kwargs
            kwargs = LLMService._prepare_litellm_kwargs(request)

            # Call LiteLLM (async)
            logger.debug(f"Calling LiteLLM with model: {request.model}")
            response = await acompletion(**kwargs)

            # Convert to our schema
            return GenerateResponse(
                id=response.id,
                object=response.object,
                created=response.created,
                model=response.model,
                choices=[
                    {
                        "index": choice.index,
                        "message": {
                            "role": choice.message.role,
                            "content": choice.message.content,
                        },
                        "finish_reason": choice.finish_reason,
                    }
                    for choice in response.choices
                ],
                usage=(
                    {
                        "prompt_tokens": response.usage.prompt_tokens,
                        "completion_tokens": response.usage.completion_tokens,
                        "total_tokens": response.usage.total_tokens,
                    }
                    if response.usage
                    else None
                ),
            )

        except Exception as e:
            logger.error(f"LiteLLM generation error: {e}")
            raise

    @staticmethod
    async def generate_stream(request: GenerateRequest) -> AsyncIterator[str]:
        """
        Generate LLM completion with streaming.

        Uses LiteLLM for unified access to all providers.

        Args:
            request: Generate request

        Yields:
            Server-Sent Events (SSE) formatted chunks

        Raises:
            Exception: If generation fails
        """
        try:
            # Prepare LiteLLM kwargs
            kwargs = LLMService._prepare_litellm_kwargs(request)
            kwargs["stream"] = True

            # Call LiteLLM (async streaming)
            logger.debug(f"Calling LiteLLM (streaming) with model: {request.model}")
            response = await acompletion(**kwargs)

            # Stream chunks as SSE
            async for chunk in response:
                # Convert chunk to JSON string
                import json
                chunk_data = {
                    "id": chunk.id,
                    "object": chunk.object,
                    "created": chunk.created,
                    "model": chunk.model,
                    "choices": [
                        {
                            "index": choice.index,
                            "delta": {
                                "role": getattr(choice.delta, "role", None),
                                "content": getattr(choice.delta, "content", None),
                            },
                            "finish_reason": choice.finish_reason,
                        }
                        for choice in chunk.choices
                    ],
                }

                # Yield as SSE format
                yield f"data: {json.dumps(chunk_data)}\n\n"

            # Send done message
            yield "data: [DONE]\n\n"

        except Exception as e:
            logger.error(f"LiteLLM streaming error: {e}")
            raise
```

### 3. Model Caching

**File**: `app/features/llm_playground/models_cache.py`

```python
"""
Model caching for LLM providers.

Mirrors the Go implementation with 15-minute TTL.
"""

import time
from typing import Dict, List, Optional, Tuple

from app.features.llm_playground.schemas import ModelInfo


class ModelsCache:
    """
    In-memory cache for provider models.

    Mirrors Go: provider/model_cache.go
    """

    def __init__(self, ttl_seconds: int = 900):  # 15 minutes default
        """
        Initialize models cache.

        Args:
            ttl_seconds: Time-to-live in seconds
        """
        self.ttl_seconds = ttl_seconds
        self._cache: Dict[str, Tuple[List[ModelInfo], float]] = {}

    def get(self, provider: str) -> Optional[List[ModelInfo]]:
        """
        Get models from cache if not expired.

        Args:
            provider: Provider name

        Returns:
            List of models if cached and not expired, None otherwise
        """
        if provider not in self._cache:
            return None

        models, timestamp = self._cache[provider]

        # Check if expired
        if time.time() - timestamp > self.ttl_seconds:
            # Expired, remove from cache
            del self._cache[provider]
            return None

        return models

    def set(self, provider: str, models: List[ModelInfo]) -> None:
        """
        Set models in cache.

        Args:
            provider: Provider name
            models: List of models to cache
        """
        self._cache[provider] = (models, time.time())

    def clear(self, provider: Optional[str] = None) -> None:
        """
        Clear cache for a provider or all providers.

        Args:
            provider: Provider name, or None to clear all
        """
        if provider:
            self._cache.pop(provider, None)
        else:
            self._cache.clear()


# Global cache instance
global_models_cache = ModelsCache(ttl_seconds=900)  # 15 minutes
```

### 4. LLM Router

**File**: `app/features/llm_playground/router.py`

```python
"""
LLM Playground router.

Unified LLM endpoints using LiteLLM.
"""

from typing import List

from fastapi import APIRouter, HTTPException, status
from fastapi.responses import StreamingResponse

from app.core.logger import logger
from app.features.auth.dependencies import CurrentUserEmail
from app.features.llm_playground.models_cache import global_models_cache
from app.features.llm_playground.schemas import (
    GenerateRequest,
    GenerateResponse,
    ModelInfo,
    ModelsResponse,
)
from app.features.llm_playground.service import LLMService
from app.features.llm_playground.utils import get_hardcoded_models

router = APIRouter()


@router.post("/generate", response_model=GenerateResponse)
async def generate(request: GenerateRequest, current_user_email: CurrentUserEmail):
    """
    Generate LLM completion (unified endpoint for all providers).

    Uses LiteLLM to route to the appropriate provider based on model name.

    Auth required: Uses CurrentUserEmail dependency.

    Args:
        request: Generation request
        current_user_email: Current user email from session (auth dependency)

    Returns:
        Generation response

    Raises:
        HTTPException: 400 for validation errors, 500 for generation failures
    """
    try:
        # Handle streaming
        if request.stream:
            async def stream_generator():
                try:
                    async for chunk in LLMService.generate_stream(request):
                        yield chunk
                except Exception as e:
                    logger.error(f"Streaming generation error: {e}")
                    yield f"data: {{\"error\": \"{str(e)}\"}}\n\n"

            return StreamingResponse(
                stream_generator(),
                media_type="text/event-stream",
            )

        # Non-streaming
        response = await LLMService.generate(request)
        return response

    except Exception as e:
        logger.error(f"Generation error: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Generation failed: {str(e)}",
        )


@router.get("/models", response_model=ModelsResponse)
async def get_models(current_user_email: CurrentUserEmail):
    """
    Get all available models across all providers.

    Auth required: Uses CurrentUserEmail dependency.

    Args:
        current_user_email: Current user email from session (auth dependency)

    Returns:
        List of all models
    """
    all_models = get_hardcoded_models()
    return ModelsResponse(models=all_models)


@router.get("/providers/{provider}/models", response_model=ModelsResponse)
async def get_models_by_provider(provider: str, current_user_email: CurrentUserEmail):
    """
    Get models for a specific provider.

    Uses 3-tier strategy: cache → API fetch → hardcoded fallback.

    Auth required: Uses CurrentUserEmail dependency.

    Args:
        provider: Provider name (openai, anthropic, gemini, etc.)
        current_user_email: Current user email from session (auth dependency)

    Returns:
        List of models for provider

    Raises:
        HTTPException: 400 if provider is invalid
    """
    # Validate provider
    valid_providers = {"openai", "anthropic", "gemini"}
    if provider not in valid_providers:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Invalid provider: {provider}",
        )

    # Tier 1: Try cache first
    cached_models = global_models_cache.get(provider)
    if cached_models:
        logger.debug(f"Using cached models for {provider}")
        return ModelsResponse(models=cached_models)

    # Tier 2: Try fetching from provider API
    # TODO: Implement API fetching (can use LiteLLM's model listing if available)

    # Tier 3: Fallback to hardcoded list
    logger.debug(f"Using hardcoded models for {provider}")
    hardcoded_models = get_hardcoded_models(provider=provider)
    return ModelsResponse(models=hardcoded_models)


@router.post("/providers/{provider}/models/refresh", response_model=ModelsResponse)
async def refresh_models(provider: str, current_user_email: CurrentUserEmail):
    """
    Force refresh models from provider API.

    Auth required: Uses CurrentUserEmail dependency.

    Args:
        provider: Provider name
        current_user_email: Current user email from session (auth dependency)

    Returns:
        Updated list of models

    Raises:
        HTTPException: 400 if provider is invalid, 500 if refresh fails
    """
    # Validate provider
    valid_providers = {"openai", "anthropic", "gemini"}
    if provider not in valid_providers:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Invalid provider: {provider}",
        )

    # TODO: Implement API fetching
    # For now, return hardcoded models
    hardcoded_models = get_hardcoded_models(provider=provider)
    global_models_cache.set(provider, hardcoded_models)

    return ModelsResponse(models=hardcoded_models)
```

### 5. Utility Functions

**File**: `app/features/llm_playground/utils.py`

```python
"""
Utility functions for LLM playground.
"""

from typing import List, Optional

from app.features.llm_playground.schemas import ModelInfo


# Hardcoded model definitions (fallback)
HARDCODED_MODELS = [
    # OpenAI Models
    ModelInfo(
        id="gpt-4o",
        provider="openai",
        display_name="GPT-4o",
        supports_structured_output=True,
        supports_vision=True,
        supports_function_calling=True,
        max_tokens=128000,
        max_output_tokens=16384,
    ),
    ModelInfo(
        id="gpt-4o-mini",
        provider="openai",
        display_name="GPT-4o Mini",
        supports_structured_output=True,
        supports_vision=True,
        supports_function_calling=True,
        max_tokens=128000,
        max_output_tokens=16384,
    ),
    ModelInfo(
        id="o1",
        provider="openai",
        display_name="OpenAI o1",
        supports_structured_output=False,
        supports_vision=False,
        supports_function_calling=False,
        max_tokens=200000,
        max_output_tokens=100000,
    ),
    ModelInfo(
        id="o1-mini",
        provider="openai",
        display_name="OpenAI o1-mini",
        supports_structured_output=False,
        supports_vision=False,
        supports_function_calling=False,
        max_tokens=128000,
        max_output_tokens=65536,
    ),
    # Anthropic Models
    ModelInfo(
        id="claude-3-7-sonnet-20250219",
        provider="anthropic",
        display_name="Claude 3.7 Sonnet",
        supports_structured_output=False,
        supports_vision=True,
        supports_function_calling=True,
        max_tokens=200000,
        max_output_tokens=8192,
    ),
    ModelInfo(
        id="claude-3-5-sonnet-20241022",
        provider="anthropic",
        display_name="Claude 3.5 Sonnet",
        supports_structured_output=False,
        supports_vision=True,
        supports_function_calling=True,
        max_tokens=200000,
        max_output_tokens=8192,
    ),
    # Gemini Models
    ModelInfo(
        id="gemini-2.0-flash-exp",
        provider="gemini",
        display_name="Gemini 2.0 Flash (Experimental)",
        supports_structured_output=True,
        supports_vision=True,
        supports_function_calling=True,
        max_tokens=1000000,
        max_output_tokens=8192,
    ),
    ModelInfo(
        id="gemini-1.5-pro",
        provider="gemini",
        display_name="Gemini 1.5 Pro",
        supports_structured_output=True,
        supports_vision=True,
        supports_function_calling=True,
        max_tokens=2000000,
        max_output_tokens=8192,
    ),
]


def get_hardcoded_models(provider: Optional[str] = None) -> List[ModelInfo]:
    """
    Get hardcoded model definitions.

    Args:
        provider: Provider name to filter by, or None for all providers

    Returns:
        List of model info
    """
    if provider:
        return [model for model in HARDCODED_MODELS if model.provider == provider]
    return HARDCODED_MODELS
```

### 6. Update Settings

**File**: `app/core/settings.py` (add LLM API keys)

```python
class Settings(BaseSettings):
    # ... existing settings ...

    # LLM API Keys (optional - for LiteLLM)
    openai_api_key: Optional[str] = None
    anthropic_api_key: Optional[str] = None
    gemini_api_key: Optional[str] = None

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
    )
```

### 7. Configure LiteLLM

**File**: `app/main.py` (configure LiteLLM)

```python
import litellm
from app.core.settings import settings

# Configure LiteLLM
def configure_litellm():
    """Configure LiteLLM with API keys and settings."""
    # Set API keys
    if settings.openai_api_key:
        litellm.openai_key = settings.openai_api_key
    if settings.anthropic_api_key:
        litellm.anthropic_key = settings.anthropic_api_key
    if settings.gemini_api_key:
        litellm.gemini_key = settings.gemini_api_key

    # Enable telemetry
    litellm.success_callback = ["opentelemetry"]
    litellm.failure_callback = ["opentelemetry"]

    # Set logging
    litellm.set_verbose = False  # Disable verbose logging


# Call in create_app()
def create_app() -> FastAPI:
    configure_litellm()

    app = FastAPI(
        title="Junjo Backend",
        lifespan=lifespan,
    )

    # Add LLM router
    from app.features.llm_playground.router import router as llm_router
    app.include_router(llm_router, prefix="/llm", tags=["llm"])

    # ... existing router setup ...

    return app
```

### 8. Update Environment Variables

**File**: `.env` (add LLM API keys)

```bash
# LLM API Keys
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...
```

## Testing Requirements

### Unit Tests

**File**: `tests/unit/features/llm_playground/test_schemas.py`

```python
"""Unit tests for LLM playground schemas."""

import pytest

from app.features.llm_playground.schemas import GenerateRequest, Message


@pytest.mark.unit
def test_generate_request_minimal():
    """Test minimal generate request."""
    request = GenerateRequest(
        model="gpt-4",
        messages=[Message(role="user", content="Hello")],
    )

    assert request.model == "gpt-4"
    assert len(request.messages) == 1
    assert request.stream is False


@pytest.mark.unit
def test_generate_request_with_parameters():
    """Test generate request with optional parameters."""
    request = GenerateRequest(
        model="gpt-4",
        messages=[Message(role="user", content="Hello")],
        temperature=0.7,
        max_tokens=100,
        top_p=0.9,
    )

    assert request.temperature == 0.7
    assert request.max_tokens == 100
    assert request.top_p == 0.9
```

**File**: `tests/unit/features/llm_playground/test_models_cache.py`

```python
"""Unit tests for models cache."""

import time

import pytest

from app.features.llm_playground.models_cache import ModelsCache
from app.features.llm_playground.schemas import ModelInfo


@pytest.mark.unit
def test_cache_set_and_get():
    """Test cache set and get."""
    cache = ModelsCache(ttl_seconds=60)
    models = [
        ModelInfo(id="gpt-4", provider="openai", display_name="GPT-4"),
    ]

    cache.set("openai", models)
    cached = cache.get("openai")

    assert cached is not None
    assert len(cached) == 1
    assert cached[0].id == "gpt-4"


@pytest.mark.unit
def test_cache_expiration():
    """Test cache expiration."""
    cache = ModelsCache(ttl_seconds=1)  # 1 second TTL
    models = [
        ModelInfo(id="gpt-4", provider="openai", display_name="GPT-4"),
    ]

    cache.set("openai", models)
    time.sleep(2)  # Wait for expiration

    cached = cache.get("openai")
    assert cached is None


@pytest.mark.unit
def test_cache_clear():
    """Test cache clear."""
    cache = ModelsCache(ttl_seconds=60)
    models = [
        ModelInfo(id="gpt-4", provider="openai", display_name="GPT-4"),
    ]

    cache.set("openai", models)
    cache.clear("openai")

    cached = cache.get("openai")
    assert cached is None
```

### Integration Tests

**File**: `tests/integration/features/llm_playground/test_llm_endpoints.py`

```python
"""Integration tests for LLM playground endpoints."""

import pytest
from httpx import AsyncClient


@pytest.mark.integration
@pytest.mark.asyncio
async def test_get_models(authenticated_client: AsyncClient, test_db_engine):
    """Test GET /llm/models endpoint."""
    response = await authenticated_client.get("/llm/models")
    assert response.status_code == 200

    data = response.json()
    assert "models" in data
    assert len(data["models"]) > 0


@pytest.mark.integration
@pytest.mark.asyncio
async def test_get_models_by_provider(authenticated_client: AsyncClient, test_db_engine):
    """Test GET /llm/providers/{provider}/models endpoint."""
    response = await authenticated_client.get("/llm/providers/openai/models")
    assert response.status_code == 200

    data = response.json()
    assert "models" in data
    assert all(model["provider"] == "openai" for model in data["models"])


@pytest.mark.integration
@pytest.mark.asyncio
async def test_generate_requires_auth(test_client: AsyncClient, test_db_engine):
    """Test that /llm/generate requires authentication."""
    response = await test_client.post(
        "/llm/generate",
        json={
            "model": "gpt-4",
            "messages": [{"role": "user", "content": "Hello"}],
        },
    )
    assert response.status_code == 401
```

## Dependencies

Add to `pyproject.toml`:

```toml
[project]
dependencies = [
    # ... existing dependencies ...
    "litellm>=1.30.0",  # Unified LLM API with OpenTelemetry support
]
```

## Phase Completion Criteria

- [ ] All files implemented and reviewed
- [ ] LiteLLM configured with API keys
- [ ] Unified `/llm/generate` endpoint works
- [ ] Streaming responses work
- [ ] Model listing works with caching
- [ ] OpenTelemetry integration works (automatic from LiteLLM)
- [ ] Error handling works correctly
- [ ] Unit tests pass (80%+ coverage)
- [ ] Integration tests pass
- [ ] Manual testing with frontend
- [ ] All providers tested (OpenAI, Anthropic, Gemini)

## Notes

1. **LiteLLM Benefits**:
   - Unified API across 100+ providers
   - Automatic OpenTelemetry instrumentation
   - Streaming support for all providers
   - Standardized error handling
   - Cost tracking and token counting

2. **Single Endpoint**: Using `/llm/generate` instead of provider-specific endpoints. LiteLLM routes to the correct provider based on model name.

3. **Model Name Format**: LiteLLM supports provider prefixes (e.g., `openai/gpt-4`, `anthropic/claude-3-opus`) or infers from model name.

4. **Streaming**: FastAPI's `StreamingResponse` with SSE format for streaming completions.

5. **API Keys**: Can be set via environment variables or settings. LiteLLM automatically uses them based on provider.

6. **Model Caching**: 15-minute TTL matching Go implementation. Future enhancement: fetch models dynamically from provider APIs.

7. **Structured Outputs**: LiteLLM supports OpenAI's structured outputs via `response_format` parameter.

8. **OpenTelemetry**: LiteLLM has built-in OpenTelemetry support. No manual instrumentation needed.

9. **Error Handling**: LiteLLM provides standardized error responses across providers.

10. **Future Enhancements**:
    - Dynamic model fetching from provider APIs
    - Function calling support
    - Vision support (multi-modal inputs)
    - Cost tracking and billing

## Migration Strategy

**Backward Compatibility**:
- Keep provider-specific endpoints initially: `/llm/openai/generate`, etc.
- These can proxy to the unified `/llm/generate` endpoint with appropriate model names
- Phase out provider-specific endpoints after frontend migration

**Frontend Changes**:
- Update to use unified `/llm/generate` endpoint
- Update model names to LiteLLM format if needed
- Update streaming response handling for SSE format

## Next Phase

Phase 8 (Deployment & Cutover) covers the strategy for transitioning from Go to Python backend in production.

"""
LLM Playground router.

Provides unified LLM generation endpoint and model discovery endpoints.
"""

from fastapi import APIRouter, HTTPException, status
import httpx
from loguru import logger

from app.features.auth.dependencies import CurrentUserEmail
from app.features.llm_playground.models import get_models_for_provider
from app.features.llm_playground.schemas import GenerateRequest, GenerateResponse, ModelsResponse
from app.features.llm_playground.service import LLMService

router = APIRouter()


@router.post("/generate", response_model=GenerateResponse)
async def generate(request: GenerateRequest, current_user_email: CurrentUserEmail):
    """
    Generate LLM completion (unified endpoint for all providers).

    LiteLLM routes to the appropriate provider based on model prefix:
    - openai/gpt-4o → OpenAI
    - anthropic/claude-3-5-sonnet → Anthropic
    - gemini/gemini-2.5-pro → Gemini

    Supports reasoning/thinking via reasoning_effort parameter (auto-translates).

    Requires authentication.

    Args:
        request: Generation request with model, messages, and parameters
        current_user_email: Current user email from session (auth dependency)

    Returns:
        Generation response with content and optional reasoning_content

    Raises:
        HTTPException: 500 if generation fails
    """
    try:
        response = await LLMService.generate(request)
        return response
    except Exception as e:
        logger.error(f"Generation error: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail=f"Generation failed: {str(e)}"
        )


@router.get("/providers/{provider}/models", response_model=ModelsResponse)
async def list_provider_models(provider: str, current_user_email: CurrentUserEmail):
    """
    List models for specific provider.

    Uses 2-tier strategy:
    1. Check cache (15-minute TTL)
    2. Fetch from provider API

    Requires authentication.

    Args:
        provider: Provider name (openai, anthropic, gemini)
        current_user_email: Current user email from session (auth dependency)

    Returns:
        List of models for provider

    Raises:
        HTTPException: 400 if provider invalid, 500 if API fails
    """
    valid_providers = {"openai", "anthropic", "gemini"}
    if provider not in valid_providers:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Invalid provider. Must be one of: {', '.join(valid_providers)}",
        )

    try:
        models = await get_models_for_provider(provider)
        return ModelsResponse(models=models)
    except ValueError as e:
        # API key not configured
        logger.error(f"Model discovery error for {provider}: {e}")
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail=str(e))
    except httpx.HTTPError as e:
        # Provider API failed
        logger.error(f"Provider API error for {provider}: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to fetch models from {provider} API",
        )
    except Exception as e:
        logger.error(f"Unexpected error fetching models for {provider}: {e}", exc_info=True)
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail=str(e))


@router.post("/providers/{provider}/models/refresh", response_model=ModelsResponse)
async def refresh_provider_models(provider: str, current_user_email: CurrentUserEmail):
    """
    Force refresh models from provider API (bypass cache).

    Requires authentication.

    Args:
        provider: Provider name (openai, anthropic, gemini)
        current_user_email: Current user email from session (auth dependency)

    Returns:
        Updated list of models

    Raises:
        HTTPException: 400 if provider invalid, 500 if API fails
    """
    valid_providers = {"openai", "anthropic", "gemini"}
    if provider not in valid_providers:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Invalid provider. Must be one of: {', '.join(valid_providers)}",
        )

    try:
        models = await get_models_for_provider(provider, force_refresh=True)
        return ModelsResponse(models=models)
    except ValueError as e:
        logger.error(f"Model refresh error for {provider}: {e}")
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail=str(e))
    except httpx.HTTPError as e:
        logger.error(f"Provider API error for {provider}: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to refresh models from {provider} API",
        )
    except Exception as e:
        logger.error(f"Unexpected error refreshing models for {provider}: {e}", exc_info=True)
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail=str(e))

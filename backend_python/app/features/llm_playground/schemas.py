"""
LLM Playground Schemas.

Pydantic models for request/response validation and serialization.
"""

from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field


class Message(BaseModel):
    """Chat message."""

    role: str = Field(..., description="Role: 'system', 'user', or 'assistant'")
    content: str = Field(..., description="Message content")


class GenerateRequest(BaseModel):
    """
    Unified LLM generation request.

    Model names use provider prefixes for LiteLLM routing:
    - openai/gpt-4o, openai/o1, openai/o3-mini
    - anthropic/claude-3-5-sonnet-20241022, anthropic/claude-3-7-sonnet-20250219
    - gemini/gemini-2.5-pro, gemini/gemini-2.5-flash
    """

    model: str = Field(..., description="Model with provider prefix")
    messages: List[Message] = Field(..., description="Chat messages")

    # Common generation parameters
    temperature: Optional[float] = Field(None, ge=0.0, le=2.0, description="Sampling temperature")
    max_tokens: Optional[int] = Field(None, gt=0, description="Maximum tokens to generate")
    top_p: Optional[float] = Field(None, ge=0.0, le=1.0, description="Nucleus sampling parameter")
    stop: Optional[List[str]] = Field(None, max_length=4, description="Stop sequences")

    # Reasoning/thinking (unified - LiteLLM translates to provider-specific)
    reasoning_effort: Optional[str] = Field(
        None,
        description="Reasoning effort: 'low', 'medium', 'high'. Auto-translates to provider thinking.",
    )

    # OpenAI-specific (for reasoning models)
    max_completion_tokens: Optional[int] = Field(
        None, gt=0, description="Max completion tokens (OpenAI reasoning models)"
    )

    # JSON mode / Structured outputs
    json_mode: bool = Field(False, description="Enable JSON output mode")
    json_schema: Optional[Dict[str, Any]] = Field(None, description="JSON schema for structured output")


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
    """
    LLM generation response (OpenAI-compatible format).

    LiteLLM returns responses in OpenAI format regardless of provider.
    """

    id: str
    object: str
    created: int
    model: str
    choices: List[Choice]
    usage: Optional[Usage] = None
    reasoning_content: Optional[str] = Field(
        None, description="Thinking/reasoning content for reasoning models"
    )


# Model discovery schemas


class ModelInfo(BaseModel):
    """Model information."""

    id: str = Field(..., description="Full model name with prefix (e.g., 'openai/gpt-4o')")
    provider: str = Field(..., description="Provider name: 'openai', 'anthropic', 'gemini'")
    display_name: str = Field(..., description="Human-readable model name")
    supports_reasoning: bool = Field(False, description="Supports reasoning/thinking")
    supports_vision: bool = Field(False, description="Supports vision/image inputs")
    max_tokens: Optional[int] = Field(None, description="Maximum context tokens")


class ModelsResponse(BaseModel):
    """Model listing response."""

    models: List[ModelInfo]

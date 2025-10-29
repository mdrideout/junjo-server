"""
LLM service using LiteLLM for unified provider access.

LiteLLM provides:
- Automatic provider routing based on model prefix (openai/, anthropic/, gemini/)
- Parameter translation (reasoning_effort → provider-specific thinking)
- Response standardization (OpenAI-compatible format)
- Automatic API key management from environment variables
"""

from typing import Any, Dict, Optional

from litellm import acompletion
from loguru import logger

from app.features.llm_playground.schemas import (
    Choice,
    GenerateRequest,
    GenerateResponse,
    Message,
    Usage,
)


class LLMService:
    """Service for LLM operations using LiteLLM."""

    @staticmethod
    def _prepare_response_format(
        json_mode: bool, json_schema: Optional[Dict[str, Any]]
    ) -> Optional[Dict[str, Any]]:
        """
        Prepare response_format parameter for LiteLLM.

        Args:
            json_mode: Whether JSON output is requested
            json_schema: Optional JSON schema for structured output

        Returns:
            response_format dict or None
        """
        if not json_mode:
            return None

        if json_schema:
            # Structured output with schema (strict mode)
            return {
                "type": "json_schema",
                "json_schema": {"name": "response_schema", "strict": True, "schema": json_schema},
            }
        else:
            # Schema-less JSON mode
            return {"type": "json_object"}

    @staticmethod
    async def generate(request: GenerateRequest) -> GenerateResponse:
        """
        Generate LLM completion using LiteLLM.

        LiteLLM automatically:
        - Routes to correct provider based on model prefix
        - Translates reasoning_effort to provider-specific thinking configs
        - Returns OpenAI-compatible response format

        Args:
            request: Generation request with model, messages, and parameters

        Returns:
            Generation response with content and optional reasoning_content

        Raises:
            Exception: If generation fails (API errors, invalid parameters, etc.)
        """
        try:
            # Prepare LiteLLM kwargs with explicit type annotation
            kwargs: Dict[str, Any] = {
                "model": request.model,
                "messages": [{"role": msg.role, "content": msg.content} for msg in request.messages],
            }

            # Add optional common parameters
            if request.temperature is not None:
                kwargs["temperature"] = request.temperature
            if request.max_tokens is not None:
                kwargs["max_tokens"] = request.max_tokens
            if request.top_p is not None:
                kwargs["top_p"] = request.top_p
            if request.stop:
                kwargs["stop"] = request.stop

            # Reasoning/thinking (LiteLLM auto-translates to provider-specific)
            if request.reasoning_effort:
                kwargs["reasoning_effort"] = request.reasoning_effort

            # OpenAI reasoning models specific parameter
            if request.max_completion_tokens is not None:
                kwargs["max_completion_tokens"] = request.max_completion_tokens

            # Response format (JSON mode / structured output)
            response_format = LLMService._prepare_response_format(request.json_mode, request.json_schema)
            if response_format:
                kwargs["response_format"] = response_format

            # Call LiteLLM (async)
            logger.info(f"LiteLLM completion request: model={request.model}")
            response = await acompletion(**kwargs)

            # Extract reasoning content if available
            # Different providers return thinking in different fields
            # Note: These fields are on the MESSAGE object, not the choice object
            reasoning_content = None
            choice = response.choices[0]
            message = choice.message  # Get message object from choice

            # OpenAI o1/o3/o4 models: reasoning_content field on message
            if hasattr(message, "reasoning_content") and message.reasoning_content:
                reasoning_content = message.reasoning_content

            # Anthropic extended thinking: thinking_blocks field on message
            elif hasattr(message, "thinking_blocks") and message.thinking_blocks and len(message.thinking_blocks) > 0:
                # thinking_blocks is a list of dicts with "thinking" field
                thinking_parts = [
                    block.get("thinking", "")
                    for block in message.thinking_blocks
                    if block.get("thinking")
                ]
                # Only set reasoning_content if we actually got text
                if thinking_parts:
                    reasoning_content = "\n".join(thinking_parts)

            # Gemini thinking: may also be in reasoning_content field on message
            # LiteLLM normalizes to reasoning_content field

            # Convert LiteLLM response to our schema
            return GenerateResponse(
                id=response.id,
                object=response.object,
                created=response.created,
                model=response.model,
                choices=[
                    Choice(
                        index=c.index,
                        message=Message(role=c.message.role, content=c.message.content or ""),
                        finish_reason=c.finish_reason,
                    )
                    for c in response.choices
                ],
                usage=(
                    Usage(
                        prompt_tokens=response.usage.prompt_tokens,
                        completion_tokens=response.usage.completion_tokens,
                        total_tokens=response.usage.total_tokens,
                    )
                    if response.usage
                    else None
                ),
                reasoning_content=reasoning_content,
            )

        except Exception as e:
            logger.error(f"LiteLLM generation error: {e}", exc_info=True)
            raise

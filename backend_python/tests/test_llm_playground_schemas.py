"""Unit tests for LLM playground schemas."""

import pytest
from pydantic import ValidationError

from app.features.llm_playground.schemas import (
    Choice,
    GenerateRequest,
    GenerateResponse,
    Message,
    ModelInfo,
    ModelsResponse,
    Usage,
)


@pytest.mark.unit
def test_message_valid():
    """Test Message schema with valid data."""
    msg = Message(role="user", content="Hello")
    assert msg.role == "user"
    assert msg.content == "Hello"


@pytest.mark.unit
def test_message_any_role():
    """Test Message schema accepts any string for role (validation delegated to LiteLLM)."""
    # Schema doesn't enforce role validation - LiteLLM handles this
    msg = Message(role="custom_role", content="Hello")
    assert msg.role == "custom_role"


@pytest.mark.unit
def test_generate_request_minimal():
    """Test GenerateRequest with minimal required fields."""
    req = GenerateRequest(
        model="openai/gpt-4o",
        messages=[Message(role="user", content="Hello")],
    )
    assert req.model == "openai/gpt-4o"
    assert len(req.messages) == 1
    assert req.temperature is None
    assert req.max_tokens is None
    assert req.reasoning_effort is None
    assert req.json_mode is False


@pytest.mark.unit
def test_generate_request_full():
    """Test GenerateRequest with all optional fields."""
    req = GenerateRequest(
        model="openai/gpt-4o",
        messages=[
            Message(role="system", content="You are helpful"),
            Message(role="user", content="Hello"),
        ],
        temperature=0.7,
        max_tokens=1000,
        top_p=0.9,
        stop=["END"],
        reasoning_effort="medium",
        max_completion_tokens=2000,
        json_mode=True,
        json_schema={"type": "object", "properties": {"name": {"type": "string"}}},
    )
    assert req.model == "openai/gpt-4o"
    assert len(req.messages) == 2
    assert req.temperature == 0.7
    assert req.max_tokens == 1000
    assert req.reasoning_effort == "medium"
    assert req.json_mode is True
    assert req.json_schema is not None


@pytest.mark.unit
def test_generate_request_any_reasoning_effort():
    """Test GenerateRequest accepts any string for reasoning_effort (validation delegated to LiteLLM)."""
    # Schema doesn't enforce reasoning_effort validation - LiteLLM handles this
    req = GenerateRequest(
        model="openai/gpt-4o",
        messages=[Message(role="user", content="Hello")],
        reasoning_effort="custom_value",
    )
    assert req.reasoning_effort == "custom_value"


@pytest.mark.unit
def test_generate_request_temperature_bounds():
    """Test GenerateRequest validates temperature bounds."""
    # Valid temperature
    req = GenerateRequest(
        model="openai/gpt-4o",
        messages=[Message(role="user", content="Hello")],
        temperature=0.5,
    )
    assert req.temperature == 0.5

    # Invalid temperature (too high)
    with pytest.raises(ValidationError):
        GenerateRequest(
            model="openai/gpt-4o",
            messages=[Message(role="user", content="Hello")],
            temperature=2.5,
        )

    # Invalid temperature (negative)
    with pytest.raises(ValidationError):
        GenerateRequest(
            model="openai/gpt-4o",
            messages=[Message(role="user", content="Hello")],
            temperature=-0.5,
        )


@pytest.mark.unit
def test_choice_schema():
    """Test Choice schema."""
    choice = Choice(
        index=0,
        message=Message(role="assistant", content="Hello!"),
        finish_reason="stop",
    )
    assert choice.index == 0
    assert choice.message.role == "assistant"
    assert choice.finish_reason == "stop"


@pytest.mark.unit
def test_usage_schema():
    """Test Usage schema."""
    usage = Usage(
        prompt_tokens=10,
        completion_tokens=20,
        total_tokens=30,
    )
    assert usage.prompt_tokens == 10
    assert usage.completion_tokens == 20
    assert usage.total_tokens == 30


@pytest.mark.unit
def test_generate_response_minimal():
    """Test GenerateResponse with minimal fields."""
    resp = GenerateResponse(
        id="chatcmpl-123",
        object="chat.completion",
        created=1234567890,
        model="openai/gpt-4o",
        choices=[
            Choice(
                index=0,
                message=Message(role="assistant", content="Response"),
                finish_reason="stop",
            )
        ],
        usage=None,
        reasoning_content=None,
    )
    assert resp.id == "chatcmpl-123"
    assert len(resp.choices) == 1
    assert resp.usage is None
    assert resp.reasoning_content is None


@pytest.mark.unit
def test_generate_response_with_reasoning():
    """Test GenerateResponse with reasoning content."""
    resp = GenerateResponse(
        id="chatcmpl-123",
        object="chat.completion",
        created=1234567890,
        model="openai/o1-preview",
        choices=[
            Choice(
                index=0,
                message=Message(role="assistant", content="Response"),
                finish_reason="stop",
            )
        ],
        usage=Usage(prompt_tokens=10, completion_tokens=20, total_tokens=30),
        reasoning_content="Let me think about this...",
    )
    assert resp.reasoning_content == "Let me think about this..."
    assert resp.usage is not None


@pytest.mark.unit
def test_model_info_schema():
    """Test ModelInfo schema."""
    model = ModelInfo(
        id="openai/gpt-4o",
        provider="openai",
        display_name="GPT-4o",
        supports_reasoning=False,
        supports_vision=True,
        max_tokens=128000,
    )
    assert model.id == "openai/gpt-4o"
    assert model.provider == "openai"
    assert model.supports_reasoning is False
    assert model.supports_vision is True


@pytest.mark.unit
def test_model_info_minimal():
    """Test ModelInfo with minimal required fields."""
    model = ModelInfo(
        id="anthropic/claude-3-5-sonnet-20241022",
        provider="anthropic",
        display_name="Claude 3.5 Sonnet",
        supports_reasoning=True,
    )
    assert model.id == "anthropic/claude-3-5-sonnet-20241022"
    assert model.supports_reasoning is True
    assert model.supports_vision is False  # Default
    assert model.max_tokens is None


@pytest.mark.unit
def test_models_response_schema():
    """Test ModelsResponse schema."""
    resp = ModelsResponse(
        models=[
            ModelInfo(
                id="openai/gpt-4o",
                provider="openai",
                display_name="GPT-4o",
                supports_reasoning=False,
            ),
            ModelInfo(
                id="anthropic/claude-3-5-sonnet-20241022",
                provider="anthropic",
                display_name="Claude 3.5 Sonnet",
                supports_reasoning=True,
            ),
        ]
    )
    assert len(resp.models) == 2
    assert resp.models[0].provider == "openai"
    assert resp.models[1].provider == "anthropic"


@pytest.mark.unit
def test_models_response_empty():
    """Test ModelsResponse with empty list."""
    resp = ModelsResponse(models=[])
    assert len(resp.models) == 0
